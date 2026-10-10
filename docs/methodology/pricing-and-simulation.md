# Pricing Models and Simulation Engine

## Overview

The pricing system answers: *"What priority fee (gwei/gas) should I set to land
in the top N positions of a block at least X% of the time?"*

It operates on `BlockFeeRecord` summaries derived from `marts.fct_blocks` and
runs entirely in the API layer — no on-chain calls at request time.

---

## Input: BlockFeeRecord

Each record summarises one tipped block:

| Field | Source | Description |
|---|---|---|
| `TopFeeGwei` | `top_fee_gwei` | Rank-1 fee in the block |
| `SecondToFirstRatio` | `second_to_first_ratio` | Rank-2 / Rank-1 (competitiveness proxy) |
| `MedianFeeGwei` | `median_fee_gwei` | Median tipped fee |
| `TippedCount` | `tipped_tx_count` | Tipped transactions in block |
| `Volatility` | (future: CEX kline) | 1-min normalised realised volatility |
| `DexSpread` | (future: DEX/CEX) | DEX–CEX spread fraction |
| `TopEntityFees` | (future: entity join) | Fees paid by top entities |

`Volatility` and `DexSpread` default to 0 until the intermediate mart joins are
wired. `TopEntityFees` defaults to empty, causing `competitor-behavior` to fall
back to `TopFeeGwei`.

### Fee at rank N

`feeAtRank(rec, n)` uses log-linear interpolation between three anchors:

- rank 1 → `TopFeeGwei`
- rank 2 → `TopFeeGwei × SecondToFirstRatio`
- rank T/2 → `MedianFeeGwei` (T = tipped count)
- rank T → `MedianFeeGwei × 0.3` (tail)

---

## Pricing Models

All models implement `Model.Recommend(ctx, Request, []BlockFeeRecord)`:

```
Request{N, Confidence, WindowBlocks}  →  Recommendation{FeeGwei, LowGwei, HighGwei, HitRate, ...}
```

### 1. recent-performance

**Empirical quantile of rank-N fees over the history window.**

1. For each block in history, compute `feeAtRank(block, N)`.
2. Take the `Confidence`-th quantile (R type-7 interpolation).
3. `LowGwei` = P10, `HighGwei` = P90 of the same distribution.

Monotone: higher confidence yields a higher or equal fee.

*Best when*: the fee distribution is stable.

---

### 2. competitor-behavior

**Recommend enough to beat the dominant competitor's observed fee.**

1. For each block, take `max(TopEntityFees)` as a proxy for the leading entity's
   bid. Falls back to `TopFeeGwei` when entity data is absent.
2. Take the `Confidence`-th quantile of those competitor fees.
3. Add a 1% beat-competitor premium.
4. `HighGwei` is at least `FeeGwei` (interval always contains recommendation).

*Best when*: a single entity dominates and its behaviour is predictable.

---

### 3. market-condition

**Adjust a baseline fee by realised volatility and DEX spread.**

1. Base = `recent-performance` at `Confidence`.
2. Multiply by `1 + VolCoef × avgVolatility + SpreadCoef × avgSpread`.
   Defaults: VolCoef = 0.30, SpreadCoef = 0.20.
3. `LowGwei` and `HighGwei` are base's P10/P90.

*Best when*: market conditions vary and CEX/DEX data is available.

*Degenerates to `recent-performance`* when both Volatility and DexSpread are 0
(current state until CEX/DEX data lands).

---

### 4. floor-estimate

**Minimum historical fee achieving confidence.**

1. For each block, compute `feeAtRank(block, N)`.
2. Sort ascending. Find the smallest `F` such that at least `Confidence`-fraction
   of blocks had `feeAtRank ≤ F`.
3. Returns a tight lower bound — the cheapest fee that has historically worked.

*Best when*: you want cost-efficiency and can tolerate occasional misses.

---

## Model Evaluation (`eval-models`)

The CLI command `pw-ingester eval-models` measures each model's actual
hit rate and average cost on a held-out test window:

```
history[0 .. TrainBlocks + TestBlocks - 1]
├── train: history[0 .. TrainBlocks-1]    (sliding window per test block)
└── test:  history[TrainBlocks ..]
```

For each test block *i*:
1. Train window = `history[i - TrainBlocks .. i - 1]`.
2. Run `Recommend(N, Confidence, WindowBlocks=TrainBlocks)`.
3. **Hit** = `FeeGwei ≥ feeAtRank(testBlock, N)`.

Results (`HitRate`, `AvgCostGwei`, `EvalBlocks`) are written to
`marts.model_eval` and exposed via `GET /api/v1/pricing/models/eval`.

---

## Simulation Engine

### Strategy interface

```go
Tip(ctx, Opportunity, recent []BlockFeeRecord) float64
```

| Strategy | Behaviour |
|---|---|
| `fixed-tip(F)` | Always bids F gwei |
| `percentile-of-recent(p)` | Bids the p-th percentile of recent `TopFeeGwei` |
| `spread-proportional(α)` | Bids α × `EstProfitGwei`; falls back to P50 if profit=0 |

### Replay engine

`Replay(ctx, ReplayConfig)` replays a sequence of `Opportunity` records:

1. For each opportunity in order, a worker goroutine queries the strategy.
2. **Win** = `tip ≥ opportunity.ActualTopFee`.
3. **P&L** (if won) = `EstProfitGwei − tip − BaseFeeGwei`.

Workers are bounded by a buffered-channel semaphore (`MaxWorkers`, default 4).
The replay is cancelled if `ctx` is cancelled.

Output:

```
ReplayResult{TotalPnL, WinRate, Wins, Total, AvgCost, CumPnL, Disclaimer}
```

**Disclaimer** (always included):
> Competitor reactions are not modelled: this replay assumes your bid does not
> change rivals' behaviour, which overstates the strategy's real-world win rate
> and P&L.

### Backtest API

| Endpoint | Purpose |
|---|---|
| `POST /api/v1/backtest` | Submit async job → `{run_id, status}` |
| `GET /api/v1/backtest/{run_id}` | Poll status |
| `GET /api/v1/backtest/{run_id}/results` | Retrieve `ReplayResult` when done |

Jobs are stored in `meta.backtest_runs` (ReplacingMergeTree). The server
maintains a bounded pool of at most 4 concurrent replay workers.

---

## Limitations

- `Volatility` and `DexSpread` default to 0 until CEX/DEX pipeline data flows
  into `BlockFeeRecord`. `market-condition` is therefore identical to
  `recent-performance` until that integration is complete.
- `TopEntityFees` is not yet populated from the entity-resolution mart.
  `competitor-behavior` falls back to `TopFeeGwei`.
- The private mempool means arrival-time ranking is unobservable. Models are
  based solely on on-chain priority fees, which are the only verifiable signal.
- Replay does not model competitor reactions, gas price feedback loops, or
  latency-adjusted arrival-time advantages.

# Arbitrage Methodology

This document defines how Priority Watch measures DEX-CEX spread,
markout P&L, and atomic-arbitrage capture rate for Arbitrum One under
the PGA ordering policy.

---

## 1. Definitions

**Atomic-arbitrage candidate** — a transaction containing ≥ 2 distinct DEX pool
swap events (`tx_class = 'atomic_arb_candidate'` in `int_tx_classification`).
This is a simplified heuristic. It does not verify net-zero token flow cycles,
which would require ERC-20 transfer data. False positives exist for complex
multi-step DeFi interactions that are not arb trades.

**CEX mid price** — the average of the Binance 1 s kline open and close prices
for the second containing the block timestamp:

$$P_{mid} = \frac{P_{open} + P_{close}}{2}$$

**Limitation**: Binance klines have 1-second granularity. Up to 4 consecutive
Arbitrum blocks (250 ms apart) share the same kline and therefore the same
reported CEX mid. Sub-second spread resolution is not available from this source.

**DEX execution price** — the price implied by the Uniswap V3 `sqrtPriceX96`
state variable emitted in the Swap event:

$$P_{dex} = \left(\frac{\text{sqrtPriceX96}}{2^{96}}\right)^2 \cdot \frac{10^{d_0}}{10^{d_1}}$$

where $d_0, d_1$ are the decimal places of token0 and token1 respectively.
This gives the price of token0 in units of token1 per token0.

For ETH/stablecoin pairs, the price is normalised to USD per ETH:

| Pool type | `exec_price_token1_per_token0` | Normalised formula |
|-----------|-------------------------------|--------------------|
| token0 = stablecoin (USD*), token1 = WETH | WETH per USD ≈ 1/3000 | $P_{usd} = 1 / P_{raw}$ |
| token0 = WETH, token1 = stablecoin | USD per WETH ≈ 3000 | $P_{usd} = P_{raw}$ |

---

## 2. CEX-DEX Spread

$$\text{spread\_bps} = \frac{P_{dex} - P_{mid}}{P_{mid}} \times 10{,}000$$

- **Positive spread**: DEX price above CEX mid — block producer can sell at
  the CEX mid and capture the spread.
- **Negative spread**: DEX price below CEX mid — arb opportunity against the
  DEX (buy cheap on-chain, sell at CEX).

Spreads are computed per block per pool in `fct_spread_per_block` and
aggregated into `agg_spread_heatmap` (pool × hour-of-day) and
`agg_spread_histogram` (bps buckets).

---

## 3. Markout P&L

For each swap, the markout at horizon $h$ seconds is:

$$\text{markout}_{h} = \Delta_{ETH} \times \left(P_{CEX}(t+h) - P_{exec}\right)$$

where:

- $\Delta_{ETH}$ = net ETH received by the trader in this swap  
  (positive = bought ETH from the pool; negative = sold ETH to the pool)
- $P_{CEX}(t+h)$ = Binance 1s kline close price $h$ seconds after the block timestamp
- $P_{exec}$ = DEX execution price normalised to USD per ETH

**Sign interpretation**:

| $\Delta_{ETH}$ | Price moved | Markout | Interpretation |
|----------------|-------------|---------|----------------|
| > 0 (bought ETH) | up | > 0 | Informed buy (bought before price rise) |
| > 0 (bought ETH) | down | < 0 | Uninformed buy |
| < 0 (sold ETH) | down | > 0 | Informed sell |
| < 0 (sold ETH) | up | < 0 | Uninformed sell |

Arb traders are expected to have large positive markouts at the 1 s and 5 s
horizons, converging toward zero at 300 s as the price impact mean-reverts.

Horizons implemented: 1 s, 5 s, 10 s, 30 s, 60 s, 300 s.
At 250 ms per block these correspond approximately to 4, 20, 40, 120, 240,
and 1 200 blocks.

---

## 4. Gross Arb Profit Estimation

For an atomic-arb-candidate transaction, the gross profit estimate is the
net USD flow to the trader across all swap legs:

$$\text{gross\_profit\_usd} = \sum_{i} -\Delta_{\text{stablecoin},i}$$

For each swap $i$:

- If token0 is a USD stablecoin: $\Delta_{\text{stablecoin}} = \text{amount0}$
  (positive = stablecoin flowing into the pool from the trader)
- If token1 is a USD stablecoin: $\Delta_{\text{stablecoin}} = \text{amount1}$

A positive gross profit means the trader net-received stablecoins — the arb
converted the DEX price discrepancy into USD profit.

**Example**: 2-leg ETH arb, buying ETH at \$2,990 and selling at \$3,010.

| Swap | amount0 (USDC) | amount1 (WETH) | stablecoin_delta |
|------|---------------:|---------------:|-----------------:|
| Buy ETH  | +2,990 | −1 | +2,990 |
| Sell ETH | −3,010 | +1 | −3,010 |

$\text{gross\_profit} = -(+2{,}990 + (-3{,}010)) = \$20$ ✓

**Limitation**: This formula only works for USD-stablecoin pairs (USDC, USDC.e,
USDT, DAI). For non-stablecoin pairs (e.g. WETH/ARB), the USD contribution
cannot be estimated without an additional ARB/USD price feed. Such pools
contribute `usd_flow = 0` to the gross profit estimate.

---

## 5. Capture Rate

$$\text{capture\_rate} = \frac{\text{tip\_paid\_usd}}{\text{gross\_profit\_usd\_est}}$$

This measures what fraction of the estimated atomic-arbitrage profit was
captured by the block producer as a priority fee (tip).

**Comparison with Entropy benchmarks** (from the PGA AIP forum thread,
March 2026, Entropy Advisors):

| Chain / Protocol | Capture rate |
|-----------------|-------------|
| Arbitrum One / Timeboost | 1.74% |
| Ethereum / PoS | 28.83% |
| Base | 7.34% |

These figures are stored in the `benchmarks` seed and rendered as reference
lines on the /opportunity capture rate chart. They use **a different
methodology** (Entropy's proprietary CEX-DEX flow analysis) and should not
be directly compared with Priority Watch's on-chain heuristic values.

Key differences:
1. Entropy use a trade-flow model to estimate arb opportunity; Priority Watch
   uses a transaction-level stablecoin-flow heuristic.
2. Entropy filter confirmed arbitrage trades; Priority Watch uses the
   `atomic_arb_candidate` heuristic (2+ distinct pool swaps, possible false
   positives).
3. Entropy's figures are based on a fixed sample period; Priority Watch
   computes daily capture rates from live chain data.

---

## 6. Failure Modes and Limitations

| Issue | Impact | Mitigation |
|-------|--------|------------|
| 1 s CEX granularity | Up to 4 blocks share the same mid price | Documented; sub-second analysis not possible |
| atomic_arb_candidate false positives | Overestimates arb count | `is_negative_profit` flag in `fct_arb_capture` for monitoring |
| Missing pool coverage | Under-estimates arb opportunity for uncovered pairs | `distinct_pools` column in `fct_arb_capture`; expand `config/pools.yaml` |
| Non-stablecoin pairs (WETH/ARB) | Gross profit underestimated | `usd_flow = 0` for these; documented in `fct_arb_capture` |
| Reorg lag | Swaps near chain tip may be re-classified | 1 h incremental overlap in all models |
| Kline gaps | NULL markouts when Binance feed gaps exist | `NULL` propagation; `meta.source_status` tracks freshness |

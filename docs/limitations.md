# Limitations

These are known limitations of Priority Watch. Every UI panel that is affected
carries a tooltip linking here.

## Private mempool

Arbitrum One operates a private mempool. Transaction arrival times, mempool
ordering, and PGA round boundaries are **not observable on-chain**. Priority Watch
can measure **outcome** (which transactions appeared in which blocks at what fees)
but cannot directly verify whether PGA ordering was applied correctly to any given
transaction.

**Impact:** The `estimated_rounds` and `ordering_score` fields in `int_block_ordering`
are heuristics based on fee gaps between consecutive transactions, not ground truth.

## 1-second CEX timestamp resolution

Binance 1s klines have 1-second granularity. Arbitrum produces ~4 blocks per second.
Up to 4 consecutive blocks share the same CEX mid price. Sub-second spread resolution
is not available from this data source.

**Impact:** `spread_bps` and `markout_usd_*` have +-500 ms price attribution error.
The heatmap and scatter charts are directionally correct but not precise enough for
intra-second arbitrage analysis.

## Heuristic entity labels

Entity labels in `config/entities.yaml` are manually curated from public sources
(on-chain behaviour, audit reports, public announcements). They may be incomplete,
stale, or incorrect. Addresses not in the registry appear as `unknown`.

**Impact:** HHI, entity position leaderboard, and payer share charts may
undercount known entities and overstate `unknown` concentration.

## Arbitrage candidate heuristic

The atomic-arbitrage candidate classifier (`tx_class = 'atomic_arb_candidate'` in
`int_tx_classification`) flags transactions with 2 or more distinct DEX pool swap events.
It does **not** verify net-zero token flow cycles. Complex multi-step DeFi
interactions (flash loans, liquidations) may be false positives.

**Impact:** Capture rate may be overstated if false positives are common. Undercounting
is also possible if arbitrage is spread across multiple transactions.

## CEX-DEX spread only measures one leg

The spread and markout calculation only observes the DEX leg. The CEX leg (if any)
is not observed. For entities that trade on both venues, the on-chain tip represents
only their marginal cost, not total P&L.

**Impact:** Capture rate is a lower bound on true arbitrage opportunity, not an
exact measure.

## No L1 fee attribution

The `l1_fee` (calldata cost) goes to `infraFeeAccount`, not `networkFeeAccount`.
Priority Watch tracks `networkFeeAccount` revenue (base fee + tip on L2 gas). L1
fees are excluded from all revenue metrics.

## Timeboost baseline

The Timeboost baseline was imported from public data. Exact match with timeboost.app
figures may differ due to block range differences and methodology divergence. See
`docs/methodology/fee-mechanics.md` for the canonical formula.

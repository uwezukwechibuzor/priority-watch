# Data Model

## Layer overview

| Layer | Database | Engine | Purpose |
|-------|----------|--------|---------|
| Raw | `raw` | ReplacingMergeTree | Verbatim on-chain data, idempotent writes |
| Meta | `meta` | ReplacingMergeTree | Ingester state, cursors, migration tracking |
| Staging | `staging` | ReplacingMergeTree | Typed, deduplicated, era-tagged |
| Intermediate | `intermediate` | ReplacingMergeTree | Derived fields, entity joins, fee attribution |
| Marts | `marts` | ReplacingMergeTree | Business aggregates, API-ready |

## Raw tables

| Table | Rows (approx) | Primary key | Notes |
|-------|---------------|-------------|-------|
| `raw.blocks` | ~35 M | `block_number` | One row per block |
| `raw.transactions` | ~450 M | `(block_number, transaction_index)` | All txs including system |
| `raw.receipts` | ~450 M | `(block_number, transaction_index)` | Gas used, status, L1 fee |
| `raw.logs` | ~800 M | `(block_number, transaction_index, log_index)` | Event logs |
| `raw.cex_klines` | ~6 M | `(symbol, open_time)` | Binance 1s ETH/USDT klines |
| `raw.balance_snapshots` | ~50 K | `(address, snapshot_block)` | Hourly fee account balances |

## Key staging models

| Model | Source | Added fields |
|-------|--------|-------------|
| `stg_blocks` | `raw.blocks` | `base_fee_per_gas_gwei`, `era`, `gas_utilisation` |
| `stg_transactions` | `raw.transactions` | `priority_fee_per_gas`, `effective_gas_price`, `tx_type_name` |
| `stg_receipts` | `raw.receipts` | `l1_fee_eth`, `l2_gas_used`, `reverted` |
| `stg_dex_swaps` | `raw.logs` | Decoded Uniswap V3/V2 swap events |

## Key intermediate models

| Model | Joins | Purpose |
|-------|-------|---------|
| `int_block_fees` | `stg_blocks` + `stg_transactions` + `stg_receipts` | Per-block fee attribution (tip, base, L1) |
| `int_entity_labels` | `stg_transactions` + seeds | Maps addresses to labelled entities |
| `int_pga_rounds` | `int_block_fees` | Block-level PGA ordering heuristics |
| `int_tx_classification` | `stg_transactions` + `stg_dex_swaps` | Labels txs: arb_candidate, liquidation, etc. |

## Key mart models

| Model | Grain | Key metrics |
|-------|-------|-------------|
| `fct_blocks` | per block | `tip_revenue_eth`, `tip_revenue_usd`, `contested`, `second_to_first_ratio` |
| `fct_tip_revenue` | per block per entity | Entity-level tip attribution |
| `fct_payer_share` | daily per entity | Rolling tip share, HHI components |
| `fct_arb_capture` | per block | `spread_opportunity_usd`, `capture_rate_pct` |
| `fct_swap_markouts` | per swap | 6-horizon markout P&L |
| `agg_health_daily` | daily | HHI, contested pct, revert rate, block count |
| `dim_entities` | per entity | Label, category, address list |

## CEX price alignment

Binance 1s klines are joined to blocks by truncating `block_timestamp` to the
nearest second. Up to 4 consecutive blocks share the same kline. See
[Limitations](limitations.md#1-second-cex-timestamp-resolution).

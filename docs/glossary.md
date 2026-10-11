# Metric Glossary

Every tile, panel, and column in the Priority Watch UI is defined here.

## Core terms

| Term | Definition | Formula | Source model | Grain | Caveats |
|------|-----------|---------|--------------|-------|---------|
| **tip** | The priority fee a sender pays | `priority_fee_per_gas x gas_used_l2` | `fct_transactions` | per-tx | Tip is on L2 gas only (H1 hypothesis — see fee mechanics) |
| **priority_fee_per_gas** | EIP-1559 priority fee rate | `min(maxPriorityFeePerGas, maxFeePerGas - baseFee)` | `stg_transactions` | per-tx | Zero for legacy type-0 txs |
| **base fee** | EIP-1559 protocol-set minimum fee | Protocol set each block | `stg_blocks` | per-block | Burned on Ethereum; flows to Treasury on Arbitrum |
| **effective_gas_price** | Total gas price paid | `baseFee + priority_fee_per_gas` | `stg_transactions` | per-tx | |
| **PGA round** | 125 ms ordering window | K = 2 rounds per 250 ms block | N/A | N/A | Not observable on-chain |
| **anti-starvation** | Virtual priority boost for waiting txs | `p/(2K)` for zero/low-tip txs | N/A | N/A | Ordering only — not charged, not visible on-chain |

## Revenue tiles

| Tile | Definition | Formula | Source model |
|------|-----------|---------|--------------|
| **REVENUE 30D** | Total tip revenue in the last 30 days | `sum(tip_revenue_eth)` | `agg_health_daily` |
| **BLOCKS 30D** | Block count vs expected | `count(block_number)` vs `(seconds_in_range / 0.25)` | `agg_health_daily` |
| **HHI 30D** | Herfindahl-Hirschman Index of tip revenue by entity | `sum((entity_share_pct)^2) x 10000` | `agg_health_daily` |
| **CONTESTED 30D** | % of blocks with 2+ distinct paying entities | `contested_blocks / total_blocks` | `agg_health_daily` |
| **TOP PAYER** | Entity with highest 30d tip share | argMax on entity tip sum | `agg_payer_share_daily` |
| **REVERT RATE** | % of txs that reverted | `reverted_count / total_count` | `fct_execution_quality_daily` |

## Block table columns

| Column | Definition | Notes |
|--------|-----------|-------|
| **BLOCK** | Block number | Links to block explorer |
| **TIME** | Block timestamp (UTC) | |
| **TIP ETH** | Total tip revenue in ETH | `tip_revenue_eth` from `fct_blocks` |
| **TIP USD** | Tip revenue in USD | `tip_revenue_usd` |
| **BASE FEE** | Base fee in Gwei | `base_fee_per_gas_gwei` |
| **TXS** | Total transaction count | Including system txs |
| **TYPE** | Block classification | `solo` (one payer), `contested` (multiple), `empty` (no tips) |
| **2ND/1ST** | Competitiveness ratio | `second_to_first_ratio`; NULL if fewer than 2 tipped txs |
| **TOP TIPPER** | Address of top-tip tx sender | `top_tipper_address` from `fct_blocks` |

## Opportunity metrics

| Metric | Definition | Formula | Source model |
|--------|-----------|---------|--------------|
| **spread_bps** | CEX-DEX price spread | `(dex_price - cex_mid) / cex_mid x 10000` | `fct_spread_per_block` |
| **markout_usd_1s** | P&L measured 1 second after execution | `exec_price - price_1s_later` (sign-adjusted) | `fct_swap_markouts` |
| **capture_rate** | Tip revenue as % of spread opportunity | `tip_revenue / spread_opportunity` | `fct_arb_capture` |
| **HHI** | Entity concentration in tip revenue | `sum(s_i^2) x 10000` where s_i = share of entity i | `agg_health_daily` |

## Treasury terms

| Term | Definition | Notes |
|------|-----------|-------|
| **NetworkFeeAccount** | Receives all priority fees from L2 gas | Address TBD from ArbOwnerPublic.GetNetworkFeeAccount() |
| **97% Treasury** | Arbitrum DAO treasury allocation | Distributed by governance every 6 months |
| **3% Developer Guild** | Arbitrum Developer Guild allocation | |

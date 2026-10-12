{{
    config(
        order_by = '(block_number, tx_hash)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Per-atomic-arb-candidate transaction: estimated gross profit, tip, and capture rate.
-- Named dataset: arb_capture (referenced by /api/v1/opportunity/capture).
--
-- gross_profit_usd methodology (see docs/methodology/arbitrage.md):
--   For ETH/stablecoin pools: Σ(-stablecoin_delta_i) across all swaps in the tx.
--   The net stablecoin received by the trader represents the arbitrage profit.
--   For non-stablecoin pairs the contribution is estimated via CEX mid (usd_flow).
--
-- capture_rate = tip_paid_usd / gross_profit_usd
--   Measures what fraction of estimated arb profit went to block producers as a tip.
--   This is comparable to Entropy's published 1.74% (Timeboost Arbitrum), 28.83% (ETH),
--   7.34% (Base) but uses a different (on-chain) methodology — see benchmarks seed.
--
-- Rows with gross_profit_usd ≤ 0 are kept but flagged; capture_rate is set to NULL.
-- These represent candidate txs where our heuristic over-classified or the arb ran at
-- a loss (rare in practice; often data quality issues like missing pool coverage).
WITH arb_swaps AS (
    SELECT
        s.block_number      AS block_number,
        s.block_timestamp   AS block_timestamp,
        s.tx_hash           AS tx_hash,
        s.log_index         AS log_index,
        s.pool_address      AS pool_address,
        s.usd_flow          AS usd_flow,
        s.eth_delta         AS eth_delta,
        s.exec_price_usd    AS exec_price_usd,
        s.era               AS era
    FROM {{ ref('fct_swap_markouts') }} AS s FINAL
    INNER JOIN {{ ref('int_tx_classification') }} AS c FINAL
        ON s.tx_hash = c.tx_hash AND s.block_number = c.block_number
    WHERE c.tx_class = 'atomic_arb_candidate'
    {% if is_incremental() %}
        AND s.block_timestamp > (
            SELECT subtractHours(max(block_timestamp), 1)
            FROM {{ this }} FINAL
        )
    {% endif %}
),

-- Aggregate per tx: sum USD flows and eth deltas across all swaps
tx_agg AS (
    SELECT
        block_number,
        block_timestamp,
        tx_hash,
        era,
        count()                             AS swap_count,
        countDistinct(pool_address)         AS distinct_pools,
        sum(usd_flow)                       AS gross_profit_usd_est,
        sum(eth_delta)                      AS net_eth_delta,
        max(exec_price_usd)                 AS max_exec_price_usd,
        min(exec_price_usd)                 AS min_exec_price_usd
    FROM arb_swaps
    GROUP BY block_number, block_timestamp, tx_hash, era
),

fees AS (
    SELECT
        block_number,
        tx_hash,
        from_address,
        tip_paid_wei,
        tip_paid_usd,
        base_fee_paid_eth,
        {{ wei_to_eth('base_fee_paid_wei') }} * eth_price_usd AS base_fee_paid_usd,
        priority_fee_per_gas_gwei,
        eth_price_usd
    FROM {{ ref('int_tx_fees') }} FINAL
)

SELECT
    ta.block_number,
    ta.block_timestamp,
    ta.tx_hash,
    ta.era,
    ta.swap_count,
    ta.distinct_pools,
    ta.gross_profit_usd_est,
    ta.net_eth_delta,
    ta.max_exec_price_usd,
    ta.min_exec_price_usd,
    -- Price spread exploited (USD/ETH difference between best and worst pool)
    ta.max_exec_price_usd - ta.min_exec_price_usd AS exploited_spread_usd,
    coalesce(f.from_address, '')        AS from_address,
    coalesce(f.tip_paid_wei, toUInt256(0)) AS tip_paid_wei,
    coalesce(f.tip_paid_usd, 0.0)       AS tip_paid_usd,
    coalesce(f.base_fee_paid_usd, 0.0)  AS base_fee_paid_usd,
    coalesce(f.priority_fee_per_gas_gwei, 0.0) AS priority_fee_per_gas_gwei,
    coalesce(f.eth_price_usd, 0.0)      AS eth_price_usd,
    -- Net profit after costs
    ta.gross_profit_usd_est
        - coalesce(f.tip_paid_usd, 0.0)
        - coalesce(f.base_fee_paid_usd, 0.0)    AS net_profit_usd,
    -- Capture rate: tip / gross arb profit
    -- NULL when gross_profit ≤ 0 (heuristic mis-classification or data gap)
    if(ta.gross_profit_usd_est > 0 AND coalesce(f.tip_paid_usd, 0.0) > 0,
       coalesce(f.tip_paid_usd, 0.0) / ta.gross_profit_usd_est,
       NULL
    ) AS capture_rate,
    -- Flag mis-classified rows for data quality monitoring
    toUInt8(ta.gross_profit_usd_est <= 0) AS is_negative_profit
FROM tx_agg ta
LEFT JOIN fees f ON ta.block_number = f.block_number AND ta.tx_hash = f.tx_hash

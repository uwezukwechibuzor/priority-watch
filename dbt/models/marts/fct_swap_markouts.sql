{{
    config(
        order_by = '(block_number, log_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Per-swap markout P&L at 6 time horizons: 1s, 5s, 10s, 30s, 60s, 300s.
-- Named dataset: swap_markouts (referenced by /api/v1/opportunity/markouts).
--
-- Markout formula (from the trader's perspective):
--   markout_usd_Ns = eth_delta × (cex_price_at_(t+N) − exec_price_usd)
-- where:
--   eth_delta = net ETH received by trader (positive = bought ETH, negative = sold ETH)
--   exec_price_usd = DEX execution price normalised to USD per ETH
--   cex_price_at_(t+N) = Binance 1s kline close price N seconds after block time
--
-- Positive markout: price moved in the trader's favour (informed trade).
-- Near-zero or negative markout: uninformed or hedged trade.
--
-- Horizon seconds correspond approximately to:
--   1s → 4 blocks, 5s → 20 blocks, 10s → 40 blocks,
--   30s → 120 blocks, 60s → 240 blocks, 300s → 1200 blocks (at 250ms/block).
--
-- Only ETH/stablecoin pools with has_usd_normalisation = 1 are included.
-- Markouts are set to NULL when the horizon kline is not yet available
-- (e.g., data at the ingestion frontier).
WITH swaps_with_exec_price AS (
    SELECT
        s.block_number,
        s.block_timestamp,
        s.tx_hash,
        s.log_index,
        s.pool_address,
        s.dex,
        s.token0_symbol,
        s.token1_symbol,
        s.cex_symbol,
        s.amount0,
        s.amount1,
        -- exec_price normalised to USD per ETH
        CASE
            WHEN position(lower(s.token0_symbol), 'usd') > 0 OR lower(s.token0_symbol) = 'dai'
                THEN if(s.exec_price_token1_per_token0 > 0,
                        1.0 / s.exec_price_token1_per_token0, 0.0)
            WHEN position(lower(s.token1_symbol), 'usd') > 0 OR lower(s.token1_symbol) = 'dai'
                THEN s.exec_price_token1_per_token0
            ELSE 0.0
        END AS exec_price_usd,
        -- eth_delta: net ETH received by the trader (positive = bought ETH)
        CASE
            WHEN lower(s.token0_symbol) = 'weth'
                THEN -s.amount0   -- trader receives WETH when amount0 < 0
            WHEN lower(s.token1_symbol) = 'weth'
                THEN -s.amount1   -- trader receives WETH when amount1 < 0
            ELSE 0.0
        END AS eth_delta,
        -- usd_flow: net USD received by the trader (for capture rate)
        CASE
            WHEN position(lower(s.token0_symbol), 'usd') > 0 OR lower(s.token0_symbol) = 'dai'
                THEN -s.amount0   -- trader receives stablecoin when amount0 < 0
            WHEN position(lower(s.token1_symbol), 'usd') > 0 OR lower(s.token1_symbol) = 'dai'
                THEN -s.amount1
            ELSE 0.0
        END AS usd_flow,
        CASE
            WHEN position(lower(s.token0_symbol), 'usd') > 0 OR lower(s.token0_symbol) = 'dai'
                THEN 1
            WHEN position(lower(s.token1_symbol), 'usd') > 0 OR lower(s.token1_symbol) = 'dai'
                THEN 1
            ELSE 0
        END AS has_usd_normalisation
    FROM {{ ref('stg_dex_swaps') }} AS s FINAL
    {% if is_incremental() %}
    WHERE s.block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
),

-- Classification join to flag arb candidates
classified AS (
    SELECT
        tx_hash,
        tx_class,
        from_address
    FROM {{ ref('int_tx_classification') }} FINAL
),

-- Fees for tip data
fees AS (
    SELECT
        block_number,
        tx_hash,
        priority_fee_per_gas_gwei,
        tip_paid_usd,
        base_fee_paid_eth,
        era
    FROM {{ ref('int_tx_fees') }} FINAL
)

SELECT
    s.block_number      AS block_number,
    s.block_timestamp   AS block_timestamp,
    s.tx_hash           AS tx_hash,
    s.log_index         AS log_index,
    s.pool_address      AS pool_address,
    concat(s.dex, ' ', s.token0_symbol, '/', s.token1_symbol) AS pool_label,
    s.cex_symbol,
    s.eth_delta,
    s.usd_flow,
    s.exec_price_usd,
    coalesce(c.tx_class, 'other')           AS tx_class,
    coalesce(c.from_address, '')            AS from_address,
    coalesce(f.priority_fee_per_gas_gwei, 0.0) AS priority_fee_per_gas_gwei,
    coalesce(f.tip_paid_usd, 0.0)           AS tip_paid_usd,
    coalesce(f.era, 'timeboost')            AS era,
    -- Horizon prices (kline close at t+N seconds); NULL when data unavailable
    k1s.price_close_usd                    AS cex_price_1s,
    k5s.price_close_usd                    AS cex_price_5s,
    k10s.price_close_usd                   AS cex_price_10s,
    k30s.price_close_usd                   AS cex_price_30s,
    k60s.price_close_usd                   AS cex_price_60s,
    k300s.price_close_usd                  AS cex_price_300s,
    -- Markout P&L in USD at each horizon
    -- Positive = price moved in the trader's favour after the trade
    if(k1s.price_close_usd > 0 AND s.exec_price_usd > 0,
       s.eth_delta * (k1s.price_close_usd - s.exec_price_usd), NULL)   AS markout_usd_1s,
    if(k5s.price_close_usd > 0 AND s.exec_price_usd > 0,
       s.eth_delta * (k5s.price_close_usd - s.exec_price_usd), NULL)   AS markout_usd_5s,
    if(k10s.price_close_usd > 0 AND s.exec_price_usd > 0,
       s.eth_delta * (k10s.price_close_usd - s.exec_price_usd), NULL)  AS markout_usd_10s,
    if(k30s.price_close_usd > 0 AND s.exec_price_usd > 0,
       s.eth_delta * (k30s.price_close_usd - s.exec_price_usd), NULL)  AS markout_usd_30s,
    if(k60s.price_close_usd > 0 AND s.exec_price_usd > 0,
       s.eth_delta * (k60s.price_close_usd - s.exec_price_usd), NULL)  AS markout_usd_60s,
    if(k300s.price_close_usd > 0 AND s.exec_price_usd > 0,
       s.eth_delta * (k300s.price_close_usd - s.exec_price_usd), NULL) AS markout_usd_300s
FROM swaps_with_exec_price s
LEFT JOIN classified c ON s.tx_hash = c.tx_hash
LEFT JOIN fees f ON s.block_number = f.block_number AND s.tx_hash = f.tx_hash
-- Horizon CEX prices via regular joins on second-aligned timestamps
LEFT JOIN {{ ref('stg_cex_klines') }} AS k1s FINAL
    ON k1s.venue = 'binance' AND k1s.symbol = s.cex_symbol
    AND k1s.open_time = toDateTime(s.block_timestamp) + INTERVAL 1 SECOND
LEFT JOIN {{ ref('stg_cex_klines') }} AS k5s FINAL
    ON k5s.venue = 'binance' AND k5s.symbol = s.cex_symbol
    AND k5s.open_time = toDateTime(s.block_timestamp) + INTERVAL 5 SECOND
LEFT JOIN {{ ref('stg_cex_klines') }} AS k10s FINAL
    ON k10s.venue = 'binance' AND k10s.symbol = s.cex_symbol
    AND k10s.open_time = toDateTime(s.block_timestamp) + INTERVAL 10 SECOND
LEFT JOIN {{ ref('stg_cex_klines') }} AS k30s FINAL
    ON k30s.venue = 'binance' AND k30s.symbol = s.cex_symbol
    AND k30s.open_time = toDateTime(s.block_timestamp) + INTERVAL 30 SECOND
LEFT JOIN {{ ref('stg_cex_klines') }} AS k60s FINAL
    ON k60s.venue = 'binance' AND k60s.symbol = s.cex_symbol
    AND k60s.open_time = toDateTime(s.block_timestamp) + INTERVAL 60 SECOND
LEFT JOIN {{ ref('stg_cex_klines') }} AS k300s FINAL
    ON k300s.venue = 'binance' AND k300s.symbol = s.cex_symbol
    AND k300s.open_time = toDateTime(s.block_timestamp) + INTERVAL 300 SECOND
WHERE s.has_usd_normalisation = 1
    AND s.cex_symbol != ''

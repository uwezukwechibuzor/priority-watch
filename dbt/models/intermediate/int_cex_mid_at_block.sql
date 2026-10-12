{{
    config(
        order_by = '(block_number, cex_symbol)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- CEX mid price matched to each block via second-aligned JOIN.
--
-- LIMITATION: stg_cex_klines has 1-second granularity, so up to 4 consecutive
-- Arbitrum blocks (250 ms each) share the same kline and therefore the same
-- reported CEX mid. Sub-second spread analysis is not possible with this source.
-- The JOIN matches on toStartOfSecond(block_timestamp) = kline open_time, which
-- is equivalent to the ASOF semantics for 1s klines (nearest kline ≤ block time).
--
-- mid_price_usd = (price_open_usd + price_close_usd) / 2 within the kline second.
-- Using open+close average rather than the close alone reduces sensitivity to
-- single-trade outliers at second boundaries.
SELECT
    b.block_number,
    b.block_timestamp,
    b.era,
    k.venue,
    k.symbol AS cex_symbol,
    k.open_time AS kline_open_time,
    k.price_open_usd,
    k.price_close_usd,
    k.price_high_usd,
    k.price_low_usd,
    (k.price_open_usd + k.price_close_usd) / 2.0  AS mid_price_usd,
    (k.price_high_usd + k.price_low_usd) / 2.0    AS hl_mid_price_usd
FROM (
    SELECT
        block_number,
        block_timestamp,
        CASE
            WHEN {{ var('pga_activation_block') }} > 0
                AND block_number >= {{ var('pga_activation_block') }}
            THEN 'pga'
            ELSE 'timeboost'
        END AS era
    FROM {{ ref('stg_blocks') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
) AS b
LEFT JOIN (
    SELECT open_time, venue, symbol, price_open_usd, price_close_usd, price_high_usd, price_low_usd
    FROM {{ ref('stg_cex_klines') }} FINAL
    WHERE venue = 'binance' AND symbol = 'ETHUSDT'
) AS k ON k.open_time = toStartOfSecond(b.block_timestamp)

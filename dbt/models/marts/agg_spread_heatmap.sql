{{
    config(
        materialized = 'table',
        order_by = '(pool_address, hour_of_day)',
        engine = 'ReplacingMergeTree'
    )
}}

-- Spread heatmap: median spread (bps) per pool × hour-of-day.
-- Powers the /opportunity Spread Heatmap panel (x = hour of day, y = pool/day-of-week).
-- Materialized as a table so the full historical pattern is always available.
SELECT
    pool_address,
    pool_label,
    cex_symbol,
    era,
    toUInt8(toHour(block_timestamp))                    AS hour_of_day,
    toUInt8(toDayOfWeek(block_timestamp))               AS day_of_week,
    -- day_of_week: 1=Monday ... 7=Sunday (ISO)
    count()                                             AS block_count,
    median(spread_bps)                                  AS median_spread_bps,
    median(abs_spread_bps)                              AS median_abs_spread_bps,
    avg(spread_bps)                                     AS avg_spread_bps,
    stddevPop(spread_bps)                               AS stddev_spread_bps,
    quantileExact(0.1)(spread_bps)                      AS p10_spread_bps,
    quantileExact(0.9)(spread_bps)                      AS p90_spread_bps
FROM {{ ref('fct_spread_per_block') }} FINAL
GROUP BY pool_address, pool_label, cex_symbol, era, hour_of_day, day_of_week

{{
    config(
        materialized = 'table',
        order_by = '(pool_address, bin_ordinal)',
        engine = 'ReplacingMergeTree'
    )
}}

-- Spread histogram: block count per spread-bps bucket per pool.
-- Powers the /opportunity Spread Histogram panel (x = spread bins, y = block count).
-- Bins in basis points (100 bps = 1%).
-- Materialized as a table for full-history histogram queries.
WITH binned AS (
    SELECT
        pool_address,
        pool_label,
        cex_symbol,
        era,
        spread_bps,
        abs_spread_bps,
        CASE
            WHEN spread_bps < -100  THEN '< -100'
            WHEN spread_bps < -50   THEN '-100 to -50'
            WHEN spread_bps < -20   THEN '-50 to -20'
            WHEN spread_bps < -10   THEN '-20 to -10'
            WHEN spread_bps < 0     THEN '-10 to 0'
            WHEN spread_bps < 10    THEN '0 to 10'
            WHEN spread_bps < 20    THEN '10 to 20'
            WHEN spread_bps < 50    THEN '20 to 50'
            WHEN spread_bps < 100   THEN '50 to 100'
            ELSE                         '> 100'
        END AS spread_bin,
        CASE
            WHEN spread_bps < -100  THEN 0
            WHEN spread_bps < -50   THEN 1
            WHEN spread_bps < -20   THEN 2
            WHEN spread_bps < -10   THEN 3
            WHEN spread_bps < 0     THEN 4
            WHEN spread_bps < 10    THEN 5
            WHEN spread_bps < 20    THEN 6
            WHEN spread_bps < 50    THEN 7
            WHEN spread_bps < 100   THEN 8
            ELSE                         9
        END AS bin_ordinal
    FROM {{ ref('fct_spread_per_block') }} FINAL
)

SELECT
    pool_address,
    pool_label,
    cex_symbol,
    era,
    spread_bin,
    bin_ordinal,
    count()                             AS block_count,
    sum(count()) OVER (
        PARTITION BY pool_address, era
        ORDER BY bin_ordinal
    )                                   AS cumulative_block_count,
    count() / sum(count()) OVER (
        PARTITION BY pool_address, era
    )                                   AS block_pct
FROM binned
GROUP BY pool_address, pool_label, cex_symbol, era, spread_bin, bin_ordinal

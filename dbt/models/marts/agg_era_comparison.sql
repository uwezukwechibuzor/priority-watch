{{
    config(
        materialized = 'table',
        order_by = 'era',
        engine = 'ReplacingMergeTree'
    )
}}

-- Era comparison: Timeboost vs PGA on identical metric definitions over the most
-- recent window_days (default 14) for each era, plus lifetime totals.
-- Used on the /health Compare tab.
-- Entropy benchmarks (capture rates) are stored as constants from the AIP thread:
--   Timeboost Arbitrum: 1.74%, Ethereum: 28.83%, Base: 7.34%
-- These are included as reference columns; computed capture rate requires
-- opportunity-layer data from task 12 (agg_opportunity_daily).
WITH window_days AS (
    SELECT {{ var('window_days', 14) }} AS days
),

daily_agg AS (
    SELECT
        date,
        era,
        tip_revenue_eth,
        tip_revenue_usd,
        tipped_tx_count,
        tx_count,
        block_count,
        expected_block_count,
        contested_share,
        hhi,
        top_payer_share,
        revert_rate_tipped,
        p50_fee_gwei
    FROM {{ ref('agg_health_daily') }} FINAL
),

-- Most recent window_days rows per era for windowed metrics
era_window AS (
    SELECT
        d.era,
        -- Windowed period boundaries
        min(d.date) AS window_start,
        max(d.date) AS window_end,
        count() AS days_in_window,
        -- Revenue
        sum(d.tip_revenue_eth)                                      AS window_tip_eth,
        avg(d.tip_revenue_eth)                                      AS avg_daily_revenue_eth,
        -- Throughput
        sum(d.tipped_tx_count)                                      AS window_tipped_txs,
        avg(d.tipped_tx_count)                                      AS avg_daily_tipped_txs,
        sum(d.block_count)                                          AS window_blocks,
        sum(d.expected_block_count)                                 AS window_expected_blocks,
        -- Competition
        avg(d.contested_share)                                      AS avg_contested_share,
        avg(toFloat64(d.hhi))                                       AS avg_hhi,
        avg(d.top_payer_share)                                      AS avg_top_payer_share,
        -- Quality
        avg(d.revert_rate_tipped)                                   AS avg_revert_rate,
        avg(d.p50_fee_gwei)                                         AS avg_median_fee_gwei
    FROM daily_agg d
    CROSS JOIN window_days wd
    WHERE d.date >= (
        SELECT subtractDays(max(date), wd.days)
        FROM daily_agg
        WHERE era = d.era
    )
    GROUP BY d.era
),

-- Lifetime totals per era
era_lifetime AS (
    SELECT
        era,
        min(date)                               AS first_date,
        max(date)                               AS last_date,
        count()                                 AS total_days,
        sum(tip_revenue_eth)                    AS lifetime_tip_eth,
        sum(tipped_tx_count)                    AS lifetime_tipped_txs,
        sum(block_count)                        AS lifetime_blocks
    FROM daily_agg
    GROUP BY era
),

-- Timeboost auction clearing prices (era = 'timeboost' only)
timeboost_totals AS (
    SELECT
        sum(timeboost_proceeds_eth)             AS total_proceeds_eth,
        count()                                 AS total_rounds
    FROM {{ ref('fct_treasury_daily') }} FINAL
    WHERE era = 'timeboost' AND timeboost_proceeds_wei > 0
)

SELECT
    ew.era,
    ew.window_start,
    ew.window_end,
    ew.days_in_window,
    ew.window_tip_eth,
    ew.avg_daily_revenue_eth,
    ew.window_tipped_txs,
    ew.avg_daily_tipped_txs,
    ew.window_blocks,
    ew.window_expected_blocks,
    if(ew.window_expected_blocks > 0,
       toFloat64(ew.window_blocks) / toFloat64(ew.window_expected_blocks),
       0.0
    ) AS blocks_pct_of_expected,
    ew.avg_contested_share,
    ew.avg_hhi,
    ew.avg_top_payer_share,
    ew.avg_revert_rate,
    ew.avg_median_fee_gwei,
    el.first_date,
    el.last_date,
    el.total_days,
    el.lifetime_tip_eth,
    el.lifetime_tipped_txs,
    el.lifetime_blocks,
    -- Timeboost auction proceeds (NULL for pga era)
    if(ew.era = 'timeboost', coalesce(tt.total_proceeds_eth, 0.0), 0.0) AS timeboost_proceeds_eth,
    if(ew.era = 'timeboost', coalesce(tt.total_rounds, toUInt64(0)), toUInt64(0)) AS timeboost_rounds,
    -- Entropy benchmarks from AIP thread (constants, not computed)
    if(ew.era = 'timeboost', 1.74, 0.0)    AS entropy_capture_rate_pct,
    28.83                                   AS entropy_capture_rate_eth_pct,
    7.34                                    AS entropy_capture_rate_base_pct,
    -- capture_rate: tip_eth / estimated_arb_opportunity_eth
    -- deferred to task 12 (opportunity marts); set to NULL until available
    toFloat64(0)                            AS capture_rate_pct,
    {{ var('window_days', 14) }}            AS window_days_var
FROM era_window ew
LEFT JOIN era_lifetime el ON ew.era = el.era
LEFT JOIN timeboost_totals tt ON ew.era = 'timeboost'

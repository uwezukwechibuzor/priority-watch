{{
    config(
        order_by = '(date, era)',
        partition_by = 'toYYYYMM(date)'
    )
}}

-- Daily health metrics for the /health Overview, Trends, and KPI tiles.
-- One row per (date, era) so the /health window selector can filter by era and date range.
--
-- zero_tip_inclusion_count: proxy for anti-starvation mechanism.
-- Counts blocks where ≥1 zero-tip tx was included in a contested block. This is a
-- documented proxy only — actual round-level ordering data is not observable on chain.
WITH daily_fees AS (
    SELECT
        toDate(f.block_timestamp)                                       AS date,
        f.era                                                          AS era,
        sum(f.tip_paid_wei)                                            AS tip_revenue_wei,
        sum(f.tip_paid_eth)                                            AS tip_revenue_eth,
        sum(f.tip_paid_usd)                                            AS tip_revenue_usd,
        count()                                                        AS tx_count,
        countIf(f.priority_fee_per_gas > 0)                           AS tipped_tx_count,
        countIf(f.priority_fee_per_gas = 0)                           AS zero_tip_tx_count,
        -- p10/p50/p90 fee bands for tipped txs only
        quantileExactIf(0.1)(f.priority_fee_per_gas_gwei, f.priority_fee_per_gas > 0)  AS p10_fee_gwei,
        quantileExactIf(0.5)(f.priority_fee_per_gas_gwei, f.priority_fee_per_gas > 0)  AS p50_fee_gwei,
        quantileExactIf(0.9)(f.priority_fee_per_gas_gwei, f.priority_fee_per_gas > 0)  AS p90_fee_gwei
    FROM {{ ref('int_tx_fees') }} AS f FINAL
    {% if is_incremental() %}
    WHERE f.block_timestamp > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY date, f.era
),

daily_spam AS (
    SELECT
        toDate(ss.block_timestamp)                              AS date,
        f.era                                                   AS era,
        sum(ss.tip_wasted_wei)                                 AS tip_wasted_wei,
        sum(toFloat64(ss.tip_wasted_wei)) / 1e18               AS tip_wasted_eth,
        countIf(ss.is_reverted_tipped = 1)                     AS reverted_tipped_count
    FROM {{ ref('int_spam_signals') }} AS ss FINAL
    LEFT JOIN (
        SELECT block_number, tx_index, era
        FROM {{ ref('int_tx_fees') }} FINAL
        {% if is_incremental() %}
        WHERE block_timestamp > (
            SELECT subtractDays(max(toDateTime(date)), 1) FROM {{ this }} FINAL
        )
        {% endif %}
    ) AS f ON ss.block_number = f.block_number AND ss.tx_index = f.tx_index
    {% if is_incremental() %}
    WHERE ss.block_timestamp > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY date, f.era
),

daily_user_cost AS (
    SELECT
        toDate(f.block_timestamp)                               AS date,
        f.era                                                   AS era,
        medianExactIf(f.total_fee_usd, c.tx_class = 'simple_transfer') AS median_simple_transfer_usd
    FROM (
        SELECT block_number, tx_index, block_timestamp, era, total_fee_usd
        FROM {{ ref('int_tx_fees') }} FINAL
        {% if is_incremental() %}
        WHERE block_timestamp > (
            SELECT subtractDays(max(toDateTime(date)), 1) FROM {{ this }} FINAL
        )
        {% endif %}
    ) AS f
    LEFT JOIN (
        SELECT block_number, tx_index, tx_class
        FROM {{ ref('int_tx_classification') }} FINAL
        {% if is_incremental() %}
        WHERE block_number IN (
            SELECT DISTINCT block_number FROM {{ ref('int_tx_fees') }} FINAL
            WHERE block_timestamp > (
                SELECT subtractDays(max(toDateTime(date)), 1) FROM {{ this }} FINAL
            )
        )
        {% endif %}
    ) AS c ON f.block_number = c.block_number AND f.tx_index = c.tx_index
    GROUP BY date, f.era
),

daily_blocks AS (
    SELECT
        toDate(b.block_timestamp)                               AS date,
        b.era,
        count()                                                AS block_count,
        345600                                                 AS expected_block_count,
        -- Proxy: blocks where zero-tip txs appeared alongside tipped txs
        countIf(b.block_type != 'untipped' AND b.tipped_tx_count < b.tx_count) AS zero_tip_inclusion_count,
        countIf(b.block_type = 'contested')                    AS contested_block_count,
        if(count() > 0,
           toFloat64(countIf(b.block_type = 'contested')) / toFloat64(count()),
           0.0
        ) AS contested_share,
        avg(toFloat64(b.distinct_tippers))                     AS avg_distinct_entities_per_block
    FROM {{ ref('fct_blocks') }} AS b FINAL
    {% if is_incremental() %}
    WHERE b.block_timestamp > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY date, b.era
),

-- Daily HHI and top payer share from entity-level aggregates
daily_entity AS (
    SELECT
        date,
        era,
        sum(toFloat64(tip_wei))                     AS total_tip_wei,
        max(toFloat64(tip_wei))                     AS top_entity_tip_wei,
        if(
            sum(toFloat64(tip_wei)) > 0,
            toUInt32(round(
                10000.0
                * sum(pow(toFloat64(tip_wei), 2))
                / pow(sum(toFloat64(tip_wei)), 2)
            )),
            toUInt32(0)
        ) AS hhi,
        if(sum(toFloat64(tip_wei)) > 0,
           max(toFloat64(tip_wei)) / sum(toFloat64(tip_wei)),
           0.0
        ) AS top_payer_share
    FROM {{ ref('agg_payer_share_daily') }} FINAL
    {% if is_incremental() %}
    WHERE toDateTime(date) > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY date, era
)

SELECT
    df.date                 AS date,
    df.era                  AS era,
    df.tip_revenue_wei      AS tip_revenue_wei,
    df.tip_revenue_eth      AS tip_revenue_eth,
    df.tip_revenue_usd      AS tip_revenue_usd,
    df.tx_count             AS tx_count,
    df.tipped_tx_count      AS tipped_tx_count,
    df.zero_tip_tx_count    AS zero_tip_tx_count,
    df.p10_fee_gwei         AS p10_fee_gwei,
    df.p50_fee_gwei         AS p50_fee_gwei,
    df.p90_fee_gwei         AS p90_fee_gwei,
    db.block_count,
    db.expected_block_count,
    toFloat64(db.block_count) / toFloat64(db.expected_block_count) AS blocks_pct_of_expected,
    db.contested_block_count,
    db.contested_share,
    db.avg_distinct_entities_per_block,
    db.zero_tip_inclusion_count,
    coalesce(de.hhi, toUInt32(0))                               AS hhi,
    CASE
        WHEN coalesce(de.hhi, toUInt32(0)) < 1500  THEN 'competitive'
        WHEN coalesce(de.hhi, toUInt32(0)) < 2500  THEN 'moderate'
        ELSE                                              'high'
    END                                                         AS hhi_band,
    coalesce(de.top_payer_share, 0.0)                           AS top_payer_share,
    if(df.tipped_tx_count > 0,
       toFloat64(coalesce(ds.reverted_tipped_count, 0)) / toFloat64(df.tipped_tx_count),
       0.0
    ) AS revert_rate_tipped,
    if(df.zero_tip_tx_count > 0,
       toFloat64(coalesce(
           -- reverted zero-tip txs (not directly available; approximate as (total_reverts - reverted_tipped))
           -- Set to 0 until we compute from int_tx_fees directly; note as TODO
           toUInt64(0)
       )) / toFloat64(df.zero_tip_tx_count),
       0.0
    ) AS revert_rate_untipped,
    coalesce(ds.tip_wasted_wei, toUInt256(0))                   AS tip_wasted_wei,
    coalesce(ds.tip_wasted_eth, 0.0)                            AS tip_wasted_eth,
    coalesce(duc.median_simple_transfer_usd, 0.0)               AS median_user_cost_usd
FROM daily_fees df
LEFT JOIN daily_blocks db ON df.date = db.date AND df.era = db.era
LEFT JOIN daily_entity de ON df.date = de.date AND df.era = de.era
LEFT JOIN daily_spam ds ON df.date = ds.date AND df.era = ds.era
LEFT JOIN daily_user_cost duc ON df.date = duc.date AND df.era = duc.era

{{
    config(
        order_by = 'hour',
        partition_by = 'toYYYYMM(hour)'
    )
}}

-- Hourly block aggregates for /blocks KPI sparklines (7d / 30d / all ranges)
-- and the TIP REVENUE / TOP FEE time-series panel.
-- expected_block_count = 14,400 (3600s / 0.25s per block).
-- HHI computed per-hour from entity tip shares within the hour.
WITH hour_entity AS (
    SELECT
        toStartOfHour(f.block_timestamp)                                AS hour,
        f.era,
        coalesce(e.entity_name, f.from_address)                        AS entity_name,
        sum(f.tip_paid_wei)                                            AS entity_tip_wei
    FROM {{ ref('int_tx_fees') }} FINAL AS f
    LEFT JOIN {{ ref('int_entities') }} FINAL AS e ON f.from_address = e.address
    WHERE f.priority_fee_per_gas > 0
    {% if is_incremental() %}
        AND f.block_timestamp > (
            SELECT subtractHours(max(hour), 1)
            FROM {{ this }} FINAL
        )
    {% endif %}
    GROUP BY hour, f.era, entity_name
),

hour_hhi AS (
    SELECT
        hour,
        era,
        sum(toFloat64(entity_tip_wei))  AS total_tip_wei,
        max(toFloat64(entity_tip_wei))  AS top_entity_tip_wei,
        if(
            sum(toFloat64(entity_tip_wei)) > 0,
            toUInt32(round(
                10000.0
                * sum(pow(toFloat64(entity_tip_wei), 2))
                / pow(sum(toFloat64(entity_tip_wei)), 2)
            )),
            toUInt32(0)
        ) AS hhi
    FROM hour_entity
    GROUP BY hour, era
),

hour_blocks AS (
    SELECT
        toStartOfHour(block_timestamp)                  AS hour,
        era,
        count()                                        AS block_count,
        14400                                          AS expected_block_count,
        sum(tip_revenue_wei)                           AS tip_revenue_wei,
        sum(tip_revenue_eth)                           AS tip_revenue_eth,
        sum(tip_revenue_usd)                           AS tip_revenue_usd,
        if(count() > 0, toFloat64(sum(tip_revenue_eth)) / toFloat64(count()), 0.0) AS avg_tip_per_block_eth,
        max(top_fee_gwei)                              AS top_fee_gwei,
        sum(tipped_tx_count)                           AS tipped_tx_count,
        countIf(block_type = 'contested')              AS contested_block_count,
        if(count() > 0,
           toFloat64(countIf(block_type = 'contested')) / toFloat64(count()),
           0.0
        ) AS contested_share
    FROM {{ ref('fct_blocks') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > (
        SELECT subtractHours(max(hour), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY hour, era
)

SELECT
    hb.hour,
    hb.era,
    hb.block_count,
    hb.expected_block_count,
    hb.tip_revenue_wei,
    hb.tip_revenue_eth,
    hb.tip_revenue_usd,
    hb.avg_tip_per_block_eth,
    hb.top_fee_gwei,
    hb.tipped_tx_count,
    hb.contested_block_count,
    hb.contested_share,
    coalesce(hh.hhi, toUInt32(0))   AS hhi,
    if(coalesce(hh.total_tip_wei, 0.0) > 0,
       coalesce(hh.top_entity_tip_wei, 0.0) / hh.total_tip_wei,
       0.0
    ) AS top_payer_share
FROM hour_blocks hb
LEFT JOIN hour_hhi hh ON hb.hour = hh.hour AND hb.era = hh.era

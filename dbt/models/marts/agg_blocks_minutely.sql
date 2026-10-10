{{
    config(
        order_by = 'minute',
        partition_by = 'toYYYYMMDD(minute)'
    )
}}

-- Minutely block aggregates for the /blocks KPI sparklines (24h range).
-- expected_block_count = 240 (60s / 0.25s per block).
-- HHI is computed per-minute from entity-level tip sums (entity shares within minute).
WITH minute_entity AS (
    SELECT
        toStartOfMinute(f.block_timestamp)                              AS minute,
        f.era,
        coalesce(e.entity_name, f.from_address)                        AS entity_name,
        sum(f.tip_paid_wei)                                            AS entity_tip_wei
    FROM {{ ref('int_tx_fees') }} FINAL AS f
    LEFT JOIN {{ ref('int_entities') }} FINAL AS e ON f.from_address = e.address
    WHERE f.priority_fee_per_gas > 0
    {% if is_incremental() %}
        AND f.block_timestamp > (
            SELECT subtractHours(max(minute), 1)
            FROM {{ this }} FINAL
        )
    {% endif %}
    GROUP BY minute, f.era, entity_name
),

minute_hhi AS (
    SELECT
        minute,
        era,
        sum(toFloat64(entity_tip_wei))  AS total_tip_wei,
        max(toFloat64(entity_tip_wei))  AS top_entity_tip_wei,
        -- HHI = 10000 * Σ(s_i^2) = 10000 * Σ(e_i^2) / T^2
        if(
            sum(toFloat64(entity_tip_wei)) > 0,
            toUInt32(round(
                10000.0
                * sum(pow(toFloat64(entity_tip_wei), 2))
                / pow(sum(toFloat64(entity_tip_wei)), 2)
            )),
            toUInt32(0)
        ) AS hhi
    FROM minute_entity
    GROUP BY minute, era
),

minute_blocks AS (
    SELECT
        toStartOfMinute(block_timestamp)                AS minute,
        era,
        count()                                        AS block_count,
        240                                            AS expected_block_count,
        sum(tip_revenue_wei)                           AS tip_revenue_wei,
        sum(tip_revenue_eth)                           AS tip_revenue_eth,
        sum(tip_revenue_usd)                           AS tip_revenue_usd,
        if(count() > 0, toFloat64(sum(tip_revenue_eth)) / toFloat64(count()), 0.0) AS avg_tip_per_block_eth,
        max(top_fee_gwei)                              AS top_fee_gwei,
        countIf(block_type = 'contested')              AS contested_block_count,
        if(count() > 0,
           toFloat64(countIf(block_type = 'contested')) / toFloat64(count()),
           0.0
        ) AS contested_share
    FROM {{ ref('fct_blocks') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > (
        SELECT subtractHours(max(minute), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY minute, era
)

SELECT
    mb.minute,
    mb.era,
    mb.block_count,
    mb.expected_block_count,
    mb.tip_revenue_wei,
    mb.tip_revenue_eth,
    mb.tip_revenue_usd,
    mb.avg_tip_per_block_eth,
    mb.top_fee_gwei,
    mb.contested_block_count,
    mb.contested_share,
    coalesce(mh.hhi, toUInt32(0))   AS hhi,
    if(coalesce(mh.total_tip_wei, 0.0) > 0,
       coalesce(mh.top_entity_tip_wei, 0.0) / mh.total_tip_wei,
       0.0
    ) AS top_payer_share
FROM minute_blocks mb
LEFT JOIN minute_hhi mh ON mb.minute = mh.minute AND mb.era = mh.era

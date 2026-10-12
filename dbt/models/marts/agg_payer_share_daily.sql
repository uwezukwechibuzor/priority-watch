{{
    config(
        order_by = '(date, entity_name)',
        partition_by = 'toYYYYMM(date)'
    )
}}

-- Daily entity-level tip revenue aggregation.
-- Powers the /blocks PAYER SHARE panel (horizontal bars) and the /health
-- PAYERS LEADERBOARD (entity sparklines = 30 daily rows per entity).
-- top_payer_share and total_tip_* at the day level are available from agg_health_daily;
-- this model is the entity-level slice.
WITH daily_entity AS (
    SELECT
        toDate(f.block_timestamp)                               AS date,
        coalesce(e.entity_name, f.from_address)                AS entity_name,
        coalesce(e.entity_type, 'unknown')                     AS entity_type,
        coalesce(e.color_index, toUInt8(0))                    AS color_index,
        coalesce(e.label_source, 'unlabelled')                 AS label_source,
        f.era,
        sum(f.tip_paid_wei)                                    AS tip_wei,
        sum(f.tip_paid_eth)                                    AS tip_eth,
        sum(f.tip_paid_usd)                                    AS tip_usd,
        count()                                                AS tx_count,
        countIf(f.priority_fee_per_gas > 0)                   AS tipped_tx_count,
        countDistinct(f.block_number)                          AS blocks_appeared,
        countIf(f.status = 0 AND f.priority_fee_per_gas > 0)  AS revert_count
    FROM {{ ref('int_tx_fees') }} AS f FINAL
    LEFT JOIN {{ ref('int_entities') }} AS e FINAL ON f.from_address = e.address
    WHERE f.priority_fee_per_gas > 0
    {% if is_incremental() %}
        AND f.block_timestamp > (
            SELECT subtractDays(max(toDateTime(date)), 1)
            FROM {{ this }} FINAL
        )
    {% endif %}
    GROUP BY date, entity_name, entity_type, color_index, label_source, f.era
),

-- Daily contested blocks per entity (appeared in a contested block)
daily_contested AS (
    SELECT
        toDate(o.block_timestamp)                               AS date,
        coalesce(e.entity_name, o.from_address)                AS entity_name,
        countDistinctIf(o.block_number, o.block_type = 'contested') AS contested_blocks
    FROM {{ ref('int_block_ordering') }} AS o FINAL
    LEFT JOIN {{ ref('int_entities') }} AS e FINAL ON o.from_address = e.address
    WHERE o.priority_fee_per_gas > 0
    {% if is_incremental() %}
        AND o.block_timestamp > (
            SELECT subtractDays(max(toDateTime(date)), 1)
            FROM {{ this }} FINAL
        )
    {% endif %}
    GROUP BY date, entity_name
),

-- Avg tip rank per entity per day
daily_rank AS (
    SELECT
        toDate(o.block_timestamp)                               AS date,
        coalesce(e.entity_name, o.from_address)                AS entity_name,
        avg(toFloat64(o.tip_rank))                             AS avg_tip_rank
    FROM {{ ref('int_block_ordering') }} AS o FINAL
    LEFT JOIN {{ ref('int_entities') }} AS e FINAL ON o.from_address = e.address
    WHERE o.priority_fee_per_gas > 0 AND o.tip_rank > 0
    {% if is_incremental() %}
        AND o.block_timestamp > (
            SELECT subtractDays(max(toDateTime(date)), 1)
            FROM {{ this }} FINAL
        )
    {% endif %}
    GROUP BY date, entity_name
)

SELECT
    de.date             AS date,
    de.entity_name      AS entity_name,
    de.entity_type      AS entity_type,
    de.color_index      AS color_index,
    de.label_source     AS label_source,
    de.era              AS era,
    de.tip_wei          AS tip_wei,
    de.tip_eth          AS tip_eth,
    de.tip_usd          AS tip_usd,
    de.tx_count         AS tx_count,
    de.tipped_tx_count  AS tipped_tx_count,
    de.blocks_appeared  AS blocks_appeared,
    coalesce(dc.contested_blocks, toUInt64(0))      AS contested_blocks,
    de.revert_count     AS revert_count,
    if(de.tipped_tx_count > 0,
       toFloat64(de.revert_count) / toFloat64(de.tipped_tx_count),
       0.0
    )                                               AS revert_rate,
    coalesce(dr.avg_tip_rank, 0.0)                  AS avg_tip_rank
FROM daily_entity de
LEFT JOIN daily_contested dc
    ON de.date = dc.date AND de.entity_name = dc.entity_name
LEFT JOIN daily_rank dr
    ON de.date = dr.date AND de.entity_name = dr.entity_name

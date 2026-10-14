{{
    config(
        order_by = '(date, entity_name)',
        partition_by = 'toYYYYMM(date)'
    )
}}

-- Per-entity per-day execution quality metrics.
-- Named dataset: execution_quality_daily (referenced by /api/v1/opportunity/execution).
-- Combines fee/revert data with swap markouts to measure execution quality.
--
-- avg_markout_Xs: average USD P&L N seconds after execution for this entity's swaps.
-- Positive = entity's trades were generally profitable (informed flow).
-- Near-zero = uninformed or perfectly hedged.
-- Negative = entity traded against the market.
WITH entity_swap_markouts AS (
    SELECT
        toDate(m.block_timestamp)                                   AS date,
        m.era,
        coalesce(nullIf(e.entity_name, ''), m.from_address)                    AS entity_name,
        coalesce(e.entity_type, 'unknown')                         AS entity_type,
        coalesce(e.color_index, toUInt8(0))                        AS color_index,
        -- Swap counts
        count()                                                    AS swap_count,
        -- Avg markouts per horizon
        avg(m.markout_usd_1s)                                      AS avg_markout_usd_1s,
        avg(m.markout_usd_5s)                                      AS avg_markout_usd_5s,
        avg(m.markout_usd_10s)                                     AS avg_markout_usd_10s,
        avg(m.markout_usd_30s)                                     AS avg_markout_usd_30s,
        avg(m.markout_usd_60s)                                     AS avg_markout_usd_60s,
        avg(m.markout_usd_300s)                                    AS avg_markout_usd_300s
    FROM {{ ref('fct_swap_markouts') }} AS m FINAL
    LEFT JOIN {{ ref('int_entities') }} AS e FINAL ON m.from_address = e.address
    {% if is_incremental() %}
    WHERE m.block_timestamp > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY date, m.era, entity_name, entity_type, color_index
),

entity_fees AS (
    SELECT
        toDate(f.block_timestamp)                                   AS date,
        coalesce(nullIf(e.entity_name, ''), f.from_address)                    AS entity_name,
        count()                                                    AS tx_count,
        countIf(f.priority_fee_per_gas > 0)                       AS tipped_tx_count,
        countIf(f.status = 0)                                     AS revert_count,
        countIf(f.status = 0 AND f.priority_fee_per_gas > 0)      AS reverted_tipped_count,
        sum(f.tip_paid_wei)                                        AS tip_paid_wei,
        sum(f.tip_paid_usd)                                        AS tip_paid_usd
    FROM {{ ref('int_tx_fees') }} AS f FINAL
    LEFT JOIN {{ ref('int_entities') }} AS e FINAL ON f.from_address = e.address
    {% if is_incremental() %}
    WHERE f.block_timestamp > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY date, entity_name
),

entity_wasted AS (
    SELECT
        toDate(ss.block_timestamp)                                  AS date,
        coalesce(nullIf(e.entity_name, ''), ss.from_address)                   AS entity_name,
        sum(ss.tip_wasted_wei)                                     AS tip_wasted_wei,
        {{ wei_to_eth('sum(ss.tip_wasted_wei)') }}                 AS tip_wasted_eth
    FROM {{ ref('int_spam_signals') }} AS ss FINAL
    LEFT JOIN {{ ref('int_entities') }} AS e FINAL ON ss.from_address = e.address
    {% if is_incremental() %}
    WHERE ss.block_timestamp > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY date, entity_name
)

SELECT
    esm.date            AS date,
    esm.era             AS era,
    esm.entity_name     AS entity_name,
    esm.entity_type     AS entity_type,
    esm.color_index     AS color_index,
    esm.swap_count      AS swap_count,
    coalesce(ef.tx_count, 0)                    AS tx_count,
    coalesce(ef.tipped_tx_count, 0)             AS tipped_tx_count,
    coalesce(ef.revert_count, 0)                AS revert_count,
    coalesce(ef.reverted_tipped_count, 0)       AS reverted_tipped_count,
    if(coalesce(ef.tipped_tx_count, 0) > 0,
       toFloat64(coalesce(ef.reverted_tipped_count, 0))
       / toFloat64(ef.tipped_tx_count), 0.0)   AS revert_rate,
    coalesce(ef.tip_paid_wei, toUInt256(0))     AS tip_paid_wei,
    coalesce(ef.tip_paid_usd, 0.0)              AS tip_paid_usd,
    coalesce(ew.tip_wasted_wei, toUInt256(0))   AS tip_wasted_wei,
    coalesce(ew.tip_wasted_eth, 0.0)            AS tip_wasted_eth,
    esm.avg_markout_usd_1s,
    esm.avg_markout_usd_5s,
    esm.avg_markout_usd_10s,
    esm.avg_markout_usd_30s,
    esm.avg_markout_usd_60s,
    esm.avg_markout_usd_300s
FROM entity_swap_markouts esm
LEFT JOIN entity_fees ef
    ON esm.date = ef.date AND esm.entity_name = ef.entity_name
LEFT JOIN entity_wasted ew
    ON esm.date = ew.date AND esm.entity_name = ew.entity_name

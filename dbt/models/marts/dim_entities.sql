{{
    config(
        materialized = 'table',
        order_by = '(entity_name, address)',
        engine = 'ReplacingMergeTree'
    )
}}

-- Entity dimension table: seed + pool + contract labels enriched with on-chain
-- lifetime stats from int_tx_fees and int_block_ordering.
-- Materialized as a table so the /health payers leaderboard can sort by lifetime_tips.
WITH lifetime_stats AS (
    SELECT
        from_address,
        min(block_number)                               AS first_seen_block,
        max(block_number)                               AS last_seen_block,
        min(block_timestamp)                            AS first_seen_ts,
        max(block_timestamp)                            AS last_seen_ts,
        count()                                         AS tx_count,
        countIf(priority_fee_per_gas > 0)              AS tipped_tx_count,
        countDistinct(block_number)                     AS distinct_blocks_appeared,
        sum(tip_paid_wei)                               AS lifetime_tip_wei,
        sum(tip_paid_eth)                               AS lifetime_tip_eth,
        countIf(status = 0 AND priority_fee_per_gas > 0) AS revert_count,
        if(countIf(priority_fee_per_gas > 0) > 0,
           toFloat64(countIf(status = 0 AND priority_fee_per_gas > 0))
           / toFloat64(countIf(priority_fee_per_gas > 0)),
           0.0
        ) AS revert_rate
    FROM {{ ref('int_tx_fees') }} FINAL
    GROUP BY from_address
),

avg_position AS (
    SELECT
        from_address,
        avgIf(toFloat64(tip_rank), tip_rank > 0) AS avg_tip_rank
    FROM {{ ref('int_block_ordering') }} FINAL
    WHERE priority_fee_per_gas > 0
    GROUP BY from_address
)

SELECT
    ie.address,
    ie.entity_name,
    ie.entity_type,
    ie.color_index,
    ie.address_label,
    ie.verified,
    ie.label_source,
    ie.confidence,
    coalesce(ls.first_seen_block, toUInt64(0))          AS first_seen_block,
    coalesce(ls.last_seen_block, toUInt64(0))           AS last_seen_block,
    coalesce(ls.first_seen_ts, toDateTime('1970-01-01')) AS first_seen_ts,
    coalesce(ls.last_seen_ts, toDateTime('1970-01-01'))  AS last_seen_ts,
    coalesce(ls.tx_count, toUInt64(0))                  AS tx_count,
    coalesce(ls.tipped_tx_count, toUInt64(0))           AS tipped_tx_count,
    coalesce(ls.distinct_blocks_appeared, toUInt64(0))  AS distinct_blocks_appeared,
    coalesce(ls.lifetime_tip_wei, toUInt256(0))         AS lifetime_tip_wei,
    coalesce(ls.lifetime_tip_eth, 0.0)                  AS lifetime_tip_eth,
    coalesce(ls.revert_count, toUInt64(0))              AS revert_count,
    coalesce(ls.revert_rate, 0.0)                       AS revert_rate,
    coalesce(ap.avg_tip_rank, 0.0)                      AS avg_tip_rank
FROM {{ ref('int_entities') }} AS ie FINAL
LEFT JOIN lifetime_stats ls ON ie.address = ls.from_address
LEFT JOIN avg_position ap ON ie.address = ap.from_address

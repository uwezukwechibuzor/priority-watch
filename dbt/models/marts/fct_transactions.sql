{{
    config(
        order_by = '(block_number, tx_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- One row per user transaction with all analytics fields joined.
-- Powers the /blocks block drawer transaction table and the full tx search.
-- Also used for per-entity aggregates in the /health payers leaderboard.
SELECT
    f.block_number              AS block_number,
    f.block_timestamp           AS block_timestamp,
    f.tx_index                  AS tx_index,
    f.tx_hash                   AS tx_hash,
    f.tx_type                   AS tx_type,
    f.from_address              AS from_address,
    f.to_address                AS to_address,
    f.value_wei                 AS value_wei,
    f.gas_used                  AS gas_used,
    f.gas_used_for_l1           AS gas_used_for_l1,
    f.gas_used_for_l2           AS gas_used_for_l2,
    f.status                    AS status,
    f.input_selector            AS input_selector,
    f.base_fee_per_gas_gwei     AS base_fee_per_gas_gwei,
    f.effective_gas_price       AS effective_gas_price,
    f.priority_fee_per_gas      AS priority_fee_per_gas,
    f.priority_fee_per_gas_gwei AS priority_fee_per_gas_gwei,
    f.tip_paid_wei              AS tip_paid_wei,
    f.tip_paid_eth              AS tip_paid_eth,
    f.tip_paid_usd              AS tip_paid_usd,
    f.total_fee_eth             AS total_fee_eth,
    f.total_fee_usd             AS total_fee_usd,
    f.eth_price_usd             AS eth_price_usd,
    f.era                       AS era,
    -- Ordering metrics from int_block_ordering
    coalesce(o.tip_rank, toUInt32(0))                           AS tip_rank,
    coalesce(o.block_type, 'untipped')                          AS block_type,
    coalesce(o.tipped_tx_count, toUInt32(0))                   AS block_tipped_tx_count,
    coalesce(o.second_to_first_ratio, 0.0)                     AS second_to_first_ratio,
    -- Classification from int_tx_classification
    coalesce(c.tx_class, 'other')                               AS tx_class,
    coalesce(c.swap_count, toUInt32(0))                        AS swap_count,
    coalesce(c.is_liquidation, toUInt8(0))                     AS is_liquidation,
    -- Spam signals from int_spam_signals
    coalesce(ss.is_reverted_tipped, toUInt8(0))                AS is_reverted_tipped,
    coalesce(ss.tip_wasted_wei, toUInt256(0))                  AS tip_wasted_wei,
    coalesce(ss.is_burst_candidate, toUInt8(0))                AS is_burst_candidate,
    -- Entity labels from int_entities
    coalesce(e.entity_name, f.from_address)                    AS entity_name,
    coalesce(e.entity_type, 'unknown')                         AS entity_type,
    coalesce(e.color_index, toUInt8(0))                        AS color_index,
    coalesce(e.label_source, 'unlabelled')                     AS label_source,
    coalesce(e.address_label, '')                              AS entity_address_label
-- Push the incremental cutoff into each large JOIN side to allow partition pruning
-- and avoid building multi-GB hash tables on a 4 GB VPS. int_entities is tiny (260 rows),
-- no filter needed there.
FROM (
    SELECT * FROM {{ ref('int_tx_fees') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > subtractHours(
        ifNull((SELECT max(block_timestamp) FROM {{ this }} FINAL),
               toDateTime64('1970-01-01 00:00:00', 3, 'UTC')),
        1
    )
    {% endif %}
) AS f
LEFT JOIN (
    SELECT block_number, tx_index, tip_rank, block_type, tipped_tx_count, second_to_first_ratio
    FROM {{ ref('int_block_ordering') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > subtractHours(
        ifNull((SELECT max(block_timestamp) FROM {{ this }} FINAL),
               toDateTime64('1970-01-01 00:00:00', 3, 'UTC')),
        1
    )
    {% endif %}
) AS o ON f.block_number = o.block_number AND f.tx_index = o.tx_index
LEFT JOIN (
    SELECT block_number, tx_index, tx_class, swap_count, is_liquidation
    FROM {{ ref('int_tx_classification') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > subtractHours(
        ifNull((SELECT max(block_timestamp) FROM {{ this }} FINAL),
               toDateTime64('1970-01-01 00:00:00', 3, 'UTC')),
        1
    )
    {% endif %}
) AS c ON f.block_number = c.block_number AND f.tx_index = c.tx_index
LEFT JOIN (
    SELECT block_number, tx_index, is_reverted_tipped, tip_wasted_wei, is_burst_candidate
    FROM {{ ref('int_spam_signals') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > subtractHours(
        ifNull((SELECT max(block_timestamp) FROM {{ this }} FINAL),
               toDateTime64('1970-01-01 00:00:00', 3, 'UTC')),
        1
    )
    {% endif %}
) AS ss ON f.block_number = ss.block_number AND f.tx_index = ss.tx_index
LEFT JOIN {{ ref('int_entities') }} AS e FINAL
    ON f.from_address = e.address

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
    f.block_number,
    f.block_timestamp,
    f.tx_index,
    f.tx_hash,
    f.tx_type,
    f.from_address,
    f.to_address,
    f.value_wei,
    f.gas_used,
    f.gas_used_for_l1,
    f.gas_used_for_l2,
    f.status,
    f.input_selector,
    f.base_fee_per_gas_gwei,
    f.effective_gas_price,
    f.priority_fee_per_gas,
    f.priority_fee_per_gas_gwei,
    f.tip_paid_wei,
    f.tip_paid_eth,
    f.tip_paid_usd,
    f.total_fee_eth,
    f.total_fee_usd,
    f.eth_price_usd,
    f.era,
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
    coalesce(e.label_source, 'unlabelled')                     AS label_source
FROM {{ ref('int_tx_fees') }} FINAL AS f
LEFT JOIN {{ ref('int_block_ordering') }} FINAL AS o
    ON f.block_number = o.block_number
    AND f.tx_index = o.tx_index
LEFT JOIN {{ ref('int_tx_classification') }} FINAL AS c
    ON f.block_number = c.block_number
    AND f.tx_index = c.tx_index
LEFT JOIN {{ ref('int_spam_signals') }} FINAL AS ss
    ON f.block_number = ss.block_number
    AND f.tx_index = ss.tx_index
LEFT JOIN {{ ref('int_entities') }} FINAL AS e
    ON f.from_address = e.address
{% if is_incremental() %}
WHERE f.block_timestamp > (
    SELECT subtractHours(max(block_timestamp), 1)
    FROM {{ this }} FINAL
)
{% endif %}

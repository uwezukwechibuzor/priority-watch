{{
    config(
        order_by = 'block_number',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- FINAL: reads deduplicated rows from the source ReplacingMergeTree.
-- argMax is not used here because FINAL on the incremental window (last 1 hour of
-- blocks) is always fast — the window is tiny relative to the full table.
SELECT
    block_number,
    block_hash,
    parent_hash,
    block_timestamp,
    l1_block_number,
    base_fee_per_gas,
    {{ to_gwei('base_fee_per_gas') }}                                   AS base_fee_per_gas_gwei,
    gas_used,
    gas_limit,
    toFloat64(gas_used) / toFloat64(nullIf(gas_limit, 0))              AS gas_utilisation,
    tx_count,
    size                                                                AS block_size_bytes,
    _ingested_at
FROM {{ source('raw', 'blocks') }} FINAL
{% if is_incremental() %}
WHERE block_timestamp > (
    SELECT subtractHours(max(block_timestamp), 1)
    FROM {{ this }} FINAL
)
{% endif %}

{{
    config(
        order_by = '(block_number, tx_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Transaction receipts with Arbitrum-specific L1/L2 gas split and derived fields.
-- gas_used_for_l2 = gas_used - gas_used_for_l1 is the L2-execution gas that
-- determines the PGA priority-fee revenue (tip = priority_fee_per_gas * gas_used_for_l2,
-- per CLAUDE.md §11 hypothesis H1).
SELECT
    block_number,
    block_timestamp,
    tx_index,
    tx_hash,
    status,
    gas_used,
    gas_used_for_l1,
    gas_used - gas_used_for_l1                                         AS gas_used_for_l2,
    effective_gas_price,
    {{ to_gwei('effective_gas_price') }}                               AS effective_gas_price_gwei,
    cumulative_gas_used,
    lower(contract_address)                                             AS contract_address,
    log_count,
    _ingested_at
FROM {{ source('raw', 'receipts') }} FINAL
{% if is_incremental() %}
WHERE block_timestamp > (
    SELECT subtractHours(max(block_timestamp), 1)
    FROM {{ this }} FINAL
)
{% endif %}

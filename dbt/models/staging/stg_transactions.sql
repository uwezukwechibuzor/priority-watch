{{
    config(
        order_by = '(block_number, tx_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- User transactions only: EIP-2718 types 0 (legacy), 1 (EIP-2930), 2 (EIP-1559).
-- Arbitrum system types (>= 100) are in stg_system_transactions.
--
-- max_fee_per_gas and max_priority_fee_per_gas are coalesced to gas_price / 0 for
-- legacy and EIP-2930 transactions so downstream models can treat all types uniformly.
-- The actual priority_fee_per_gas computation (which also needs base_fee from blocks)
-- is deferred to the intermediate layer.
SELECT
    block_number,
    block_timestamp,
    tx_index,
    tx_hash,
    tx_type,
    lower(from_address)                                     AS from_address,
    lower(to_address)                                       AS to_address,
    nonce,
    value                                                   AS value_wei,
    gas_limit,
    gas_price,
    coalesce(max_fee_per_gas, gas_price)                    AS max_fee_per_gas,
    coalesce(max_priority_fee_per_gas, toUInt256(0))        AS max_priority_fee_per_gas,
    input_selector,
    input_size,
    _ingested_at
FROM {{ source('raw', 'transactions') }} FINAL
WHERE tx_type IN (0, 1, 2)
{% if is_incremental() %}
    AND block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
{% endif %}

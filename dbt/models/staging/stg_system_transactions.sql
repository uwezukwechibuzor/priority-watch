{{
    config(
        order_by = '(block_number, tx_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Arbitrum system transactions (tx_type >= 100 / 0x64).
-- These are internal Nitro transactions (e.g. L1→L2 messages, retryables, fee collection)
-- that do not originate from user wallets and do not pay priority fees.
-- Separated from stg_transactions so user-fee models can exclude them cheaply with a
-- single source reference rather than a WHERE filter on the combined table.
SELECT
    block_number,
    block_timestamp,
    tx_index,
    tx_hash,
    tx_type,
    lower(from_address)     AS from_address,
    lower(to_address)       AS to_address,
    nonce,
    value                   AS value_wei,
    gas_limit,
    gas_price,
    input_selector,
    input_size,
    _ingested_at
FROM {{ source('raw', 'transactions') }} FINAL
WHERE tx_type >= 100
{% if is_incremental() %}
    AND block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
{% endif %}

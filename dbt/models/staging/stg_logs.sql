{{
    config(
        order_by = '(block_number, log_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Normalised event logs from all configured contract/pool addresses.
-- Raw topics and data fields are preserved as hex for decoding in derived models.
-- All addresses and topics are lower-cased for consistent JOIN keys.
SELECT
    block_number,
    block_timestamp,
    tx_index,
    tx_hash,
    log_index,
    lower(address)      AS address,
    lower(topic0)       AS topic0,
    lower(topic1)       AS topic1,
    lower(topic2)       AS topic2,
    lower(topic3)       AS topic3,
    data,
    _ingested_at
FROM {{ source('raw', 'logs') }} FINAL
{% if is_incremental() %}
WHERE block_timestamp > (
    SELECT subtractHours(max(block_timestamp), 1)
    FROM {{ this }} FINAL
)
{% endif %}

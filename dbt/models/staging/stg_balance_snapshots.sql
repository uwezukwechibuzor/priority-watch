{{
    config(
        order_by = '(address, block_number)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Hourly ETH balance snapshots for fee-recipient addresses
-- (NetworkFeeAccount and InfraFeeAccount, filled in task 8 part C).
-- balance_eth is Float64 for display; use balance_wei for accounting.
SELECT
    lower(address)              AS address,
    block_number,
    block_timestamp,
    balance_wei,
    {{ wei_to_eth('balance_wei') }} AS balance_eth,
    _ingested_at
FROM {{ source('raw', 'balance_snapshots') }} FINAL
{% if is_incremental() %}
WHERE block_timestamp > (
    SELECT subtractHours(max(block_timestamp), 1)
    FROM {{ this }} FINAL
)
{% endif %}

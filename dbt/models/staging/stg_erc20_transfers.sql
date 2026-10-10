{{
    config(
        order_by = '(block_number, log_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Decoded ERC-20 Transfer events from raw.logs.
-- topic0 = keccak256("Transfer(address,address,uint256)")
--        = 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef
--
-- NOTE: raw.logs is filtered by the ingester to only contain logs from addresses
-- in config/contracts.yaml and config/pools.yaml.  Token contracts (WETH, USDC, etc.)
-- are NOT currently in the filter, so this model will be empty until token contract
-- addresses are added to the ingester address filter.  The model is included for
-- forward compatibility and to document the intended transformation.
--
-- ABI layout:
--   topic1 (indexed): from  address (0-padded to 32 bytes)
--   topic2 (indexed): to    address
--   data:             value uint256
SELECT
    block_number,
    block_timestamp,
    tx_hash,
    log_index,
    lower(address)                                          AS token_address,
    lower(concat('0x', substring(topic1, 27)))              AS from_address,
    lower(concat('0x', substring(topic2, 27)))              AS to_address,
    {{ hex_to_uint256(
        'if(startsWith(data, \'0x\'), substring(data, 3), data)'
    ) }}                                                    AS value_raw,
    _ingested_at
FROM {{ source('raw', 'logs') }} FINAL
WHERE lower(topic0) = lower('{{ var("erc20_transfer_topic0") }}')
{% if is_incremental() %}
    AND block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
{% endif %}

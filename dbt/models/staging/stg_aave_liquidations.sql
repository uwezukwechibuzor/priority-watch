{{
    config(
        order_by = '(block_number, log_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Decoded Aave V3 LiquidationCall events from raw.logs.
-- Source: AaveV3Pool contract (0x794a61358d6845594f94dc1db02a252b5b4814ad).
-- topic0 = keccak256("LiquidationCall(address,address,address,uint256,uint256,address,bool)")
--        = 0xe413a321e8681d831f4dbccbeca18d89968b851ef0d8e3e8d1bf2af7c1c6efdf
--
-- ABI layout (topics are indexed, data is non-indexed):
--   topic1 (indexed):  collateralAsset  address (right-aligned in 32 bytes)
--   topic2 (indexed):  debtAsset        address
--   topic3 (indexed):  user             address
--
--   data field (ABI-encoded, big-endian, hex after stripping 0x):
--   bytes 0-31   (hex 1-64):    debtToCover                  uint256
--   bytes 32-63  (hex 65-128):  liquidatedCollateralAmount   uint256
--   bytes 64-95  (hex 129-192): liquidator                   address (left-padded to 32 bytes)
--   bytes 96-127 (hex 193-256): receiveAToken                bool    (right-padded to 32 bytes)
WITH raw_liquidations AS (
    SELECT
        block_number,
        block_timestamp,
        tx_hash,
        log_index,
        topic1,
        topic2,
        topic3,
        if(startsWith(data, '0x'), substring(data, 3), data) AS clean_data,
        _ingested_at
    FROM {{ source('raw', 'logs') }} FINAL
    WHERE lower(topic0) = lower('{{ var("aave_liquidation_topic0") }}')
    {% if is_incremental() %}
        AND block_timestamp > (
            SELECT subtractHours(max(block_timestamp), 1)
            FROM {{ this }} FINAL
        )
    {% endif %}
)

SELECT
    block_number,
    block_timestamp,
    tx_hash,
    log_index,
    -- Address fields: topic is 0x + 24 zero padding + 40-char address
    lower(concat('0x', substring(topic1, 27)))  AS collateral_asset,
    lower(concat('0x', substring(topic2, 27)))  AS debt_asset,
    lower(concat('0x', substring(topic3, 27)))  AS user_address,
    -- Data fields
    {{ hex_to_uint256('substring(clean_data, 1, 64)') }}                AS debt_to_cover_wei,
    {{ hex_to_uint256('substring(clean_data, 65, 64)') }}               AS liquidated_collateral_amount_wei,
    lower(concat('0x', substring(clean_data, 153, 40)))                 AS liquidator,
    -- receiveAToken: last byte of the 32-byte slot (hex chars 255-256)
    toUInt8(reinterpretAsUInt8(unhex(substring(clean_data, 255, 2))))   AS receive_a_token,
    _ingested_at
FROM raw_liquidations

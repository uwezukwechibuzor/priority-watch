{{
    config(
        order_by = '(block_number, log_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Decoded Uniswap V3 and Algebra (Camelot V3) Swap events.
-- Both protocols share topic0 0xc42079...ca67 because their Swap event ABI signatures
-- are identical; pool address distinguishes protocol (via pools seed join).
--
-- Data field layout (ABI-encoded, big-endian, no 0x prefix after stripping):
--   bytes 0-31  (hex 1-64):    amount0   int256
--   bytes 32-63 (hex 65-128):  amount1   int256
--   bytes 64-95 (hex 129-192): sqrtPriceX96 uint160
--   bytes 96-127(hex 193-256): liquidity uint128
--   bytes 128-159(hex 257-320):tick      int24
--
-- exec_price_token1_per_token0 = (sqrtPriceX96/2^96)^2 * 10^decimals0 / 10^decimals1
-- Sign: positive amount1 means token1 flowing in → selling token0 for token1.
--
-- Decimal-adjusted amounts require the pools seed; run `dbt seed` before this model.
WITH raw_swaps AS (
    SELECT
        block_number,
        block_timestamp,
        tx_hash,
        log_index,
        lower(address)                                          AS pool_address,
        topic1,
        topic2,
        if(startsWith(data, '0x'), substring(data, 3), data)   AS clean_data,
        _ingested_at
    FROM {{ source('raw', 'logs') }} FINAL
    WHERE lower(topic0) = lower('{{ var("uniswap_v3_swap_topic0") }}')
    {% if is_incremental() %}
        AND block_timestamp > (
            SELECT subtractHours(max(block_timestamp), 1)
            FROM {{ this }} FINAL
        )
    {% endif %}
),

decoded AS (
    SELECT
        block_number,
        block_timestamp,
        tx_hash,
        log_index,
        pool_address,
        lower(concat('0x', substring(topic1, 27)))              AS sender,
        lower(concat('0x', substring(topic2, 27)))              AS recipient,
        {{ hex_to_int256('substring(clean_data, 1, 64)')   }}   AS amount0_raw,
        {{ hex_to_int256('substring(clean_data, 65, 64)')  }}   AS amount1_raw,
        {{ hex_to_uint256('substring(clean_data, 129, 64)')}}   AS sqrt_price_x96,
        _ingested_at
    FROM raw_swaps
)

SELECT
    d.block_number,
    d.block_timestamp,
    d.tx_hash,
    d.log_index,
    d.pool_address,
    d.sender,
    d.recipient,
    d.amount0_raw,
    d.amount1_raw,
    d.sqrt_price_x96,
    coalesce(p.dex, 'unknown')                                              AS dex,
    coalesce(p.token0_symbol, 'TOKEN0')                                     AS token0_symbol,
    coalesce(toUInt8(p.token0_decimals), toUInt8(18))                       AS token0_decimals,
    coalesce(p.token1_symbol, 'TOKEN1')                                     AS token1_symbol,
    coalesce(toUInt8(p.token1_decimals), toUInt8(18))                       AS token1_decimals,
    coalesce(p.fee, toUInt32(0))                                            AS pool_fee,
    coalesce(p.cex_symbol, '')                                              AS cex_symbol,
    -- Decimal-adjusted token flow amounts (negative = out of pool, positive = into pool)
    toFloat64(d.amount0_raw) / pow(10.0, toFloat64(coalesce(toUInt8(p.token0_decimals), toUInt8(18))))  AS amount0,
    toFloat64(d.amount1_raw) / pow(10.0, toFloat64(coalesce(toUInt8(p.token1_decimals), toUInt8(18))))  AS amount1,
    -- Execution price: token1 per token0, human-readable, decimal-adjusted.
    -- Computed from sqrtPriceX96: price = (sqrt/2^96)^2 * 10^d0 / 10^d1
    pow(toFloat64(d.sqrt_price_x96) / pow(2.0, 96.0), 2.0)
        * pow(10.0, toFloat64(coalesce(toUInt8(p.token0_decimals), toUInt8(18))))
        / pow(10.0, toFloat64(coalesce(toUInt8(p.token1_decimals), toUInt8(18))))                       AS exec_price_token1_per_token0,
    d._ingested_at
FROM decoded d
LEFT JOIN {{ ref('pools') }} p
    ON d.pool_address = lower(p.address)
    AND p.address != ''

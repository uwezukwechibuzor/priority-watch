{{
    config(
        order_by = '(block_number, pool_address)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Last DEX execution price per pool per block.
-- Uses argMax over log_index to pick the final swap in the block for each pool,
-- which is the closest DEX price to the end-of-block state.
--
-- dex_price_usd: execution price normalised to USD per ETH for all ETH/stablecoin
-- pools. Normalisation rules:
--   token0 = stablecoin (USD*): dex_price_usd = 1 / exec_price_token1_per_token0
--   token1 = stablecoin (USD*): dex_price_usd = exec_price_token1_per_token0
--   no stablecoin leg (e.g. WETH/ARB): dex_price_usd = NULL (no normalisation)
--
-- Stablecoin detection: token symbol contains 'USD' or equals 'DAI'.
SELECT
    block_number,
    block_timestamp,
    pool_address,
    dex,
    token0_symbol,
    token1_symbol,
    cex_symbol,
    -- argMax picks the last swap's price in each block
    argMax(exec_price_token1_per_token0, log_index)     AS last_exec_price_token1_per_token0,
    argMax(amount0, log_index)                          AS last_amount0,
    argMax(amount1, log_index)                          AS last_amount1,
    count()                                             AS swap_count_in_block,
    -- Normalise to USD per ETH for stablecoin pairs
    CASE
        WHEN position(lower(token0_symbol), 'usd') > 0 OR lower(token0_symbol) = 'dai'
            THEN if(
                    argMax(exec_price_token1_per_token0, log_index) > 0,
                    1.0 / argMax(exec_price_token1_per_token0, log_index),
                    0.0
                 )
        WHEN position(lower(token1_symbol), 'usd') > 0 OR lower(token1_symbol) = 'dai'
            THEN argMax(exec_price_token1_per_token0, log_index)
        ELSE 0.0
    END AS dex_price_usd,
    -- Flag for whether normalisation was possible
    CASE
        WHEN position(lower(token0_symbol), 'usd') > 0 OR lower(token0_symbol) = 'dai'
            THEN 1
        WHEN position(lower(token1_symbol), 'usd') > 0 OR lower(token1_symbol) = 'dai'
            THEN 1
        ELSE 0
    END AS has_usd_normalisation
FROM {{ ref('stg_dex_swaps') }} FINAL
{% if is_incremental() %}
WHERE block_timestamp > (
    SELECT subtractHours(max(block_timestamp), 1)
    FROM {{ this }} FINAL
)
{% endif %}
GROUP BY
    block_number,
    block_timestamp,
    pool_address,
    dex,
    token0_symbol,
    token1_symbol,
    cex_symbol

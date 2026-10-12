{{
    config(
        order_by = '(block_number, tx_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Transaction classification for PGA analytics.
-- Priority order (highest first): liquidation > atomic_arb_candidate > dex_swap >
-- simple_transfer > other.
--
-- atomic_arb_candidate heuristic: tx_hash appears in 2+ distinct swap events
-- (simplified cycle proxy — does not verify net-zero token flow, which requires
-- ERC-20 transfer data unavailable at this layer).
--
-- simple_transfer heuristic: ETH send (value_wei > 0, input_size <= 4) or
-- ERC-20 Transfer (input_selector = 0xa9059cbb) from a non-contract address.
WITH swap_counts AS (
    SELECT
        block_number,
        tx_hash,
        countDistinct(pool_address) AS swap_count
    FROM {{ ref('stg_dex_swaps') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ ref('int_tx_fees') }} FINAL
    )
    {% endif %}
    GROUP BY block_number, tx_hash
),

liquidation_txs AS (
    SELECT DISTINCT
        block_number,
        tx_hash
    FROM {{ ref('stg_aave_liquidations') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ ref('int_tx_fees') }} FINAL
    )
    {% endif %}
)

SELECT
    f.block_number      AS block_number,
    f.block_timestamp   AS block_timestamp,
    f.tx_index          AS tx_index,
    f.tx_hash           AS tx_hash,
    f.from_address      AS from_address,
    f.to_address        AS to_address,
    f.input_selector    AS input_selector,
    f.input_size        AS input_size,
    f.value_wei         AS value_wei,
    coalesce(sc.swap_count, 0)                                              AS swap_count,
    toUInt8(liq.tx_hash IS NOT NULL)                                        AS is_liquidation,
    -- Final classification (highest-priority rule wins)
    CASE
        WHEN liq.tx_hash IS NOT NULL
            THEN 'liquidation'
        WHEN coalesce(sc.swap_count, 0) >= 2
            THEN 'atomic_arb_candidate'
        WHEN coalesce(sc.swap_count, 0) = 1
            THEN 'dex_swap'
        WHEN (
            -- Plain ETH transfer: no meaningful calldata, some ETH value
            (f.value_wei > 0 AND f.input_size <= 4)
            -- ERC-20 transfer(address,uint256)
            OR f.input_selector = '0xa9059cbb'
        )
            THEN 'simple_transfer'
        ELSE 'other'
    END AS tx_class,
    f.era               AS era
FROM {{ ref('int_tx_fees') }} AS f FINAL
LEFT JOIN swap_counts sc
    ON f.block_number = sc.block_number
    AND f.tx_hash = sc.tx_hash
LEFT JOIN liquidation_txs liq
    ON f.block_number = liq.block_number
    AND f.tx_hash = liq.tx_hash
{% if is_incremental() %}
WHERE f.block_timestamp > (
    SELECT subtractHours(max(block_timestamp), 1)
    FROM {{ this }} FINAL
)
{% endif %}

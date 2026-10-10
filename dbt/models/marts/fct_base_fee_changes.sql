{{
    config(
        order_by = 'block_number',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Per-block base fee with delta from the previous block.
-- Used for the BASE FEE EVOLUTION panel on /health.
-- The EIP-1559 base fee adjusts ±12.5% per block based on gas utilisation.
-- BaseFeeManager contract events (if configured) are not currently decoded;
-- min_base_fee is carried as NULL until that source is available.
WITH ordered AS (
    SELECT
        block_number,
        block_timestamp,
        base_fee_per_gas,
        base_fee_per_gas_gwei,
        gas_used,
        gas_limit,
        gas_utilisation,
        lagInFrame(base_fee_per_gas, 1, base_fee_per_gas) OVER (
            ORDER BY block_number
        ) AS prev_base_fee,
        era
    FROM (
        SELECT
            b.block_number,
            b.block_timestamp,
            b.base_fee_per_gas,
            b.base_fee_per_gas_gwei,
            b.gas_used,
            b.gas_limit,
            b.gas_utilisation,
            CASE
                WHEN {{ var('pga_activation_block') }} > 0
                    AND b.block_number >= {{ var('pga_activation_block') }}
                THEN 'pga'
                ELSE 'timeboost'
            END AS era
        FROM {{ ref('stg_blocks') }} FINAL AS b
        {% if is_incremental() %}
        WHERE b.block_timestamp > (
            SELECT subtractHours(max(block_timestamp), 1)
            FROM {{ this }} FINAL
        )
        {% endif %}
    )
)

SELECT
    block_number,
    block_timestamp,
    base_fee_per_gas,
    base_fee_per_gas_gwei,
    gas_used,
    gas_limit,
    gas_utilisation,
    toInt256(base_fee_per_gas) - toInt256(prev_base_fee)                AS base_fee_delta_wei,
    {{ to_gwei('toInt256(base_fee_per_gas) - toInt256(prev_base_fee)') }} AS base_fee_delta_gwei,
    if(prev_base_fee > 0,
       (toFloat64(base_fee_per_gas) - toFloat64(prev_base_fee)) / toFloat64(prev_base_fee) * 100.0,
       0.0
    ) AS base_fee_change_pct,
    -- Theoretical max change ±12.5% per block (EIP-1559 mechanism)
    if(gas_limit > 0,
       (toFloat64(gas_used) / toFloat64(gas_limit) - 0.5) / 0.5 * 0.125 * 100.0,
       0.0
    ) AS theoretical_change_pct,
    era
FROM ordered

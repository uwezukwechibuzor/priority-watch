{{
    config(
        order_by = 'hour',
        partition_by = 'toYYYYMM(hour)'
    )
}}

-- Hourly base fee statistics for the BASE FEE EVOLUTION panel on /health.
SELECT
    toStartOfHour(block_timestamp)          AS hour,
    era,
    count()                                 AS block_count,
    min(base_fee_per_gas_gwei)              AS min_base_fee_gwei,
    max(base_fee_per_gas_gwei)              AS max_base_fee_gwei,
    avg(base_fee_per_gas_gwei)              AS avg_base_fee_gwei,
    medianExact(base_fee_per_gas_gwei)      AS median_base_fee_gwei,
    -- Net base fee change within the hour
    toFloat64(max(base_fee_per_gas)) - toFloat64(min(base_fee_per_gas)) AS range_wei,
    -- % change from first to last block in hour
    if(min(base_fee_per_gas) > 0,
       (toFloat64(argMax(base_fee_per_gas, block_number))
        - toFloat64(argMin(base_fee_per_gas, block_number)))
       / toFloat64(argMin(base_fee_per_gas, block_number)) * 100.0,
       0.0
    ) AS hour_change_pct
FROM {{ ref('fct_base_fee_changes') }} FINAL
{% if is_incremental() %}
WHERE block_timestamp > (
    SELECT subtractHours(max(hour), 1)
    FROM {{ this }} FINAL
)
{% endif %}
GROUP BY hour, era

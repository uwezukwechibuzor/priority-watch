{{
    config(
        order_by = 'block_number',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- One row per block. Powers the /blocks block table, KPI tiles and sparklines.
-- Block-level ordering stats (block_type, second_to_first_ratio, ordering_score,
-- estimated_rounds) are lifted from int_block_ordering by taking the first tx in
-- each block (all rows in a block carry the same block-level aggregates).
WITH incremental_cutoff AS (
    SELECT
    {% if is_incremental() %}
        subtractHours(max(block_timestamp), 1)
    {% else %}
        toDateTime('1970-01-01 00:00:00')
    {% endif %}
    AS cutoff
    FROM {{ this }} FINAL
    LIMIT 1
),

-- Per-block aggregates from transaction fees
block_tx_agg AS (
    SELECT
        f.block_number,
        f.block_timestamp,
        f.era,
        max(f.eth_price_usd)                                                    AS eth_price_usd,
        count()                                                                 AS tx_count,
        countIf(f.priority_fee_per_gas > 0)                                    AS tipped_tx_count,
        sumIf(f.tip_paid_wei, f.priority_fee_per_gas > 0)                      AS tip_revenue_wei,
        sumIf(f.tip_paid_eth, f.priority_fee_per_gas > 0)                      AS tip_revenue_eth,
        sumIf(f.tip_paid_usd, f.priority_fee_per_gas > 0)                      AS tip_revenue_usd,
        maxIf(f.priority_fee_per_gas_gwei, f.priority_fee_per_gas > 0)         AS top_fee_gwei,
        maxIf(f.tip_paid_usd, f.priority_fee_per_gas > 0)                      AS top_tip_usd,
        quantileExactIf(0.5)(f.priority_fee_per_gas_gwei, f.priority_fee_per_gas > 0) AS median_fee_gwei,
        quantileExactIf(0.9)(f.priority_fee_per_gas_gwei, f.priority_fee_per_gas > 0) AS p90_fee_gwei,
        argMaxIf(f.from_address, f.tip_paid_wei, f.priority_fee_per_gas > 0)  AS top_payer_address,
        countIf(f.status = 0 AND f.priority_fee_per_gas > 0)                  AS revert_count
    FROM {{ ref('int_tx_fees') }} FINAL AS f
    CROSS JOIN incremental_cutoff AS ic
    WHERE f.block_timestamp > ic.cutoff
    GROUP BY f.block_number, f.block_timestamp, f.era
),

-- Block-level ordering stats lifted from the first tx in each block
block_order_stats AS (
    SELECT
        block_number,
        argMin(second_to_first_ratio, tx_index)     AS second_to_first_ratio,
        argMin(block_type, tx_index)                AS block_type,
        argMin(ordering_score, tx_index)            AS ordering_score,
        argMin(estimated_rounds, tx_index)          AS estimated_rounds,
        argMin(has_anomaly, tx_index)               AS has_anomaly,
        argMin(tipped_tx_count, tx_index)           AS bo_tipped_tx_count,
        argMin(distinct_tippers, tx_index)          AS distinct_tippers
    FROM {{ ref('int_block_ordering') }} FINAL
    CROSS JOIN incremental_cutoff AS ic
    WHERE block_timestamp > ic.cutoff
    GROUP BY block_number
),

-- Base block metadata
block_meta AS (
    SELECT
        block_number,
        base_fee_per_gas,
        base_fee_per_gas_gwei,
        gas_used,
        gas_limit,
        gas_utilisation
    FROM {{ ref('stg_blocks') }} FINAL
    CROSS JOIN incremental_cutoff AS ic
    WHERE block_timestamp > ic.cutoff
)

SELECT
    a.block_number,
    a.block_timestamp,
    a.era,
    a.tx_count,
    a.tipped_tx_count,
    a.tip_revenue_wei,
    a.tip_revenue_eth,
    a.tip_revenue_usd,
    a.eth_price_usd,
    a.top_fee_gwei,
    a.top_tip_usd,
    a.median_fee_gwei,
    a.p90_fee_gwei,
    a.top_payer_address,
    coalesce(e.entity_name, a.top_payer_address)    AS top_payer_name,
    coalesce(e.color_index, toUInt8(0))             AS top_payer_color_index,
    a.revert_count,
    coalesce(o.second_to_first_ratio, 0.0)          AS second_to_first_ratio,
    coalesce(o.block_type, 'untipped')              AS block_type,
    coalesce(o.ordering_score, 0.0)                 AS ordering_score,
    coalesce(o.estimated_rounds, toUInt32(0))       AS estimated_rounds,
    coalesce(o.has_anomaly, toUInt8(0))             AS anomaly_count,
    coalesce(o.distinct_tippers, toUInt32(0))       AS distinct_tippers,
    m.base_fee_per_gas,
    m.base_fee_per_gas_gwei,
    m.gas_used,
    m.gas_limit,
    m.gas_utilisation                               AS gas_used_pct
FROM block_tx_agg a
LEFT JOIN block_order_stats o ON a.block_number = o.block_number
LEFT JOIN block_meta m ON a.block_number = m.block_number
LEFT JOIN {{ ref('int_entities') }} FINAL AS e ON a.top_payer_address = e.address

{{
    config(
        order_by = '(date, entity_name)',
        partition_by = 'toYYYYMM(date)'
    )
}}

-- Per-entity daily ordering efficiency metrics.
-- Replaces timeboost.app's controller latency leaderboard for PGA.
-- Powers the /opportunity Entity Position Leaderboard.
--
-- delta = avg_tip_rank - avg_tx_index_among_tipped:
--   > 0: entity pays enough to rank N but lands at a higher index (worse position)
--         — possible ordering inefficiency or private mempool effects
--   < 0: entity consistently lands earlier than their fee rank suggests
--         — advantaged arrival time or bundler priority
--   ≈ 0: ordering matches fee ranking (expected under PGA)
--
-- fee_efficiency = top_3_positions_won / total_tips_gwei:
--   positions won in the first 3 slots per gwei spent (higher = more efficient)
SELECT
    toDate(f.block_timestamp)                                       AS date,
    f.era,
    coalesce(e.entity_name, f.from_address)                        AS entity_name,
    coalesce(e.entity_type, 'unknown')                             AS entity_type,
    coalesce(e.color_index, toUInt8(0))                            AS color_index,
    count()                                                        AS tx_count,
    countIf(f.priority_fee_per_gas > 0)                           AS tipped_tx_count,
    -- Average priority fee paid (gwei)
    avgIf(f.priority_fee_per_gas_gwei, f.priority_fee_per_gas > 0) AS avg_fee_gwei,
    -- Average position in block (0-based tx_index among tipped txs)
    avgIf(toFloat64(f.tx_index), f.priority_fee_per_gas > 0)      AS avg_tx_index,
    -- Average fee-based rank (1 = highest fee in block; 0 = untipped excluded)
    avgIf(toFloat64(o.tip_rank), o.tip_rank > 0)                  AS avg_tip_rank,
    -- Delta: how many positions behind/ahead of fee-rank the entity typically lands
    -- Positive = consistently behind expected position (ordered less favourably)
    avgIf(toFloat64(o.tip_rank), o.tip_rank > 0)
        - avgIf(toFloat64(f.tx_index), f.priority_fee_per_gas > 0) AS delta_rank_vs_position,
    -- Share of appearances where the tx landed in the first 3 positions of the block
    if(countIf(f.priority_fee_per_gas > 0) > 0,
       toFloat64(countIf(f.tx_index < 3 AND f.priority_fee_per_gas > 0))
       / toFloat64(countIf(f.priority_fee_per_gas > 0)),
       0.0
    ) AS top3_position_share,
    -- Fee efficiency: top-3 position wins per total gwei spent
    if(sumIf(f.priority_fee_per_gas_gwei, f.priority_fee_per_gas > 0) > 0,
       toFloat64(countIf(f.tx_index < 3 AND f.priority_fee_per_gas > 0))
       / sumIf(f.priority_fee_per_gas_gwei, f.priority_fee_per_gas > 0),
       0.0
    ) AS fee_efficiency,
    -- Block count and contested share for context
    countDistinct(f.block_number)                                  AS blocks_appeared,
    countDistinctIf(f.block_number, o.block_type = 'contested')   AS contested_blocks_appeared,
    -- Tip revenue
    sumIf(f.tip_paid_usd, f.priority_fee_per_gas > 0)             AS tip_paid_usd,
    sumIf(f.tip_paid_wei, f.priority_fee_per_gas > 0)             AS tip_paid_wei
FROM {{ ref('int_tx_fees') }} FINAL AS f
LEFT JOIN {{ ref('int_block_ordering') }} FINAL AS o
    ON f.block_number = o.block_number AND f.tx_index = o.tx_index
LEFT JOIN {{ ref('int_entities') }} FINAL AS e
    ON f.from_address = e.address
{% if is_incremental() %}
WHERE f.block_timestamp > (
    SELECT subtractDays(max(toDateTime(date)), 1)
    FROM {{ this }} FINAL
)
{% endif %}
GROUP BY date, f.era, entity_name, entity_type, color_index

{{
    config(
        order_by = '(block_number, tx_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Spam and revert signals for tipped transactions.
-- is_reverted_tipped: tx paid a tip but reverted (fee wasted on failed execution).
-- is_burst_candidate: tx belongs to a burst — same (entity, selector, block) group
--   with 3+ reverts, indicating possible over-bidding / duplicate-intent firing.
-- Only tipped transactions are included (priority_fee_per_gas > 0).
WITH burst_counts AS (
    SELECT
        f.from_address,
        f.to_address,
        f.input_selector,
        f.block_number,
        countIf(f.status = 0) AS revert_count
    FROM {{ ref('int_tx_fees') }} FINAL AS f
    WHERE f.priority_fee_per_gas > 0
        AND f.to_address != ''
        AND f.input_selector != ''
    {% if is_incremental() %}
        AND f.block_timestamp > (
            SELECT subtractHours(max(block_timestamp), 1)
            FROM {{ this }} FINAL
        )
    {% endif %}
    GROUP BY
        f.from_address,
        f.to_address,
        f.input_selector,
        f.block_number
    HAVING revert_count >= 3
)

SELECT
    f.block_number,
    f.block_timestamp,
    f.tx_index,
    f.tx_hash,
    f.from_address,
    f.to_address,
    f.input_selector,
    f.priority_fee_per_gas,
    f.tip_paid_wei,
    f.status,
    f.era,
    toUInt8(f.status = 0)                                   AS is_reverted_tipped,
    -- tip_wasted_wei: full tip even on reverts (EVM still charges gas for reverts)
    if(f.status = 0, f.tip_paid_wei, toUInt256(0))          AS tip_wasted_wei,
    toUInt8(bc.from_address IS NOT NULL)                    AS is_burst_candidate
FROM {{ ref('int_tx_fees') }} FINAL AS f
LEFT JOIN burst_counts bc
    ON f.from_address = bc.from_address
    AND f.to_address = bc.to_address
    AND f.input_selector = bc.input_selector
    AND f.block_number = bc.block_number
WHERE f.priority_fee_per_gas > 0
{% if is_incremental() %}
    AND f.block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
{% endif %}

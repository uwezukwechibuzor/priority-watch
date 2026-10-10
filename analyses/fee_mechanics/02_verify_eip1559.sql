-- Verifies the EIP-1559 formula for post-activation type-2 transactions:
--   priority_fee_per_gas = min(max_priority_fee_per_gas, max_fee_per_gas - base_fee)
--   effective_gas_price  = base_fee + priority_fee_per_gas
--
-- Replace PGA_ACTIVATION_BLOCK with the value from 01_find_activation_block.sql.

WITH
    activation AS (SELECT 0 AS block),  -- replace 0 with PGA_ACTIVATION_BLOCK
    sample AS (
        SELECT
            t.tx_hash,
            t.block_number,
            b.base_fee_per_gas,
            r.effective_gas_price,
            assumeNotNull(t.max_fee_per_gas)          AS max_fee,
            assumeNotNull(t.max_priority_fee_per_gas) AS max_tip
        FROM raw.transactions t FINAL
        JOIN raw.receipts r FINAL ON r.tx_hash = t.tx_hash
        JOIN raw.blocks   b FINAL ON b.block_number = t.block_number
        CROSS JOIN activation
        WHERE t.tx_type = 2
          AND t.max_fee_per_gas IS NOT NULL
          AND t.max_priority_fee_per_gas IS NOT NULL
          AND t.block_number >= activation.block
        LIMIT 1000000
    )
SELECT
    count()                                                      AS total_txs,
    countIf(
        r_effective = computed_effective
    )                                                            AS formula_match,
    round(
        countIf(r_effective = computed_effective) * 100.0 / count(), 4
    )                                                            AS match_pct,
    count() - countIf(r_effective = computed_effective)          AS mismatches
FROM (
    SELECT
        effective_gas_price                                            AS r_effective,
        base_fee_per_gas + least(max_tip, max_fee - base_fee_per_gas) AS computed_effective
    FROM sample
);

-- Break down by tx type (0 = legacy, 1 = access list, 2 = EIP-1559)
SELECT
    t.tx_type,
    count()                                     AS txs,
    countIf(r.effective_gas_price = b.base_fee_per_gas)  AS zero_tip_txs,
    countIf(r.effective_gas_price > b.base_fee_per_gas)  AS nonzero_tip_txs,
    toString(avg(toFloat64(r.effective_gas_price) - toFloat64(b.base_fee_per_gas)))
                                                AS avg_priority_fee_wei
FROM raw.transactions t FINAL
JOIN raw.receipts r FINAL ON r.tx_hash = t.tx_hash
JOIN raw.blocks   b FINAL ON b.block_number = t.block_number
CROSS JOIN (SELECT 0 AS block) AS activation  -- replace 0
WHERE t.block_number >= activation.block
GROUP BY t.tx_type
ORDER BY t.tx_type;

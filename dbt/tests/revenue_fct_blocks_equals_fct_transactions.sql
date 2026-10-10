-- Revenue in fct_blocks must equal the sum of tip_paid_wei in fct_transactions per day.
-- Returns rows where the discrepancy exceeds 1 wei (floating-point rounding tolerance).
WITH blocks_daily AS (
    SELECT
        toDate(block_timestamp) AS date,
        sum(tip_revenue_wei)    AS block_tip_wei
    FROM {{ ref('fct_blocks') }} FINAL
    GROUP BY date
),

txs_daily AS (
    SELECT
        toDate(block_timestamp) AS date,
        sum(tip_paid_wei)       AS tx_tip_wei
    FROM {{ ref('fct_transactions') }} FINAL
    WHERE priority_fee_per_gas > 0
    GROUP BY date
)

SELECT
    b.date,
    b.block_tip_wei,
    t.tx_tip_wei,
    toInt256(b.block_tip_wei) - toInt256(coalesce(t.tx_tip_wei, toUInt256(0))) AS diff_wei
FROM blocks_daily b
LEFT JOIN txs_daily t ON b.date = t.date
WHERE abs(toInt256(b.block_tip_wei) - toInt256(coalesce(t.tx_tip_wei, toUInt256(0)))) > 1

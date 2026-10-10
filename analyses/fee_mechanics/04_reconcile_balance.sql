-- Reconciles computed tip against observed balance change over a 1,000-block window.
--
-- Two hypotheses tested:
--   H1: tip = priority_fee_per_gas * (gas_used - gas_used_for_l1)   [tip on L2 gas only]
--   H2: tip = priority_fee_per_gas * gas_used                        [tip on all gas]
--
-- Replace START_BLOCK, END_BLOCK, and RECIPIENT_ADDR with verified values.

WITH
    window AS (
        SELECT
            0 AS start_block,   -- replace
            0 AS end_block,     -- replace: start_block + 1000
            '' AS recipient     -- replace: network_fee_acct or treasury
    )

SELECT
    sum(
        (toFloat64(r.effective_gas_price) - toFloat64(b.base_fee_per_gas))
        * toFloat64(r.gas_used - r.gas_used_for_l1)
    ) / 1e18                   AS h1_tip_eth,

    sum(
        (toFloat64(r.effective_gas_price) - toFloat64(b.base_fee_per_gas))
        * toFloat64(r.gas_used)
    ) / 1e18                   AS h2_tip_eth,

    sum(
        toFloat64(r.effective_gas_price) * toFloat64(r.gas_used)
    ) / 1e18                   AS total_fee_eth,

    sum(
        toFloat64(b.base_fee_per_gas) * toFloat64(r.gas_used)
    ) / 1e18                   AS base_fee_component_eth,

    -- observed balance delta (from raw.balance_snapshots) to be substituted:
    0.0                        AS observed_balance_delta_eth,

    -- residuals (fill observed_balance_delta_eth to evaluate):
    abs(h1_tip_eth - observed_balance_delta_eth) / nullIfZero(observed_balance_delta_eth)
                               AS h1_residual_pct,
    abs(h2_tip_eth - observed_balance_delta_eth) / nullIfZero(observed_balance_delta_eth)
                               AS h2_residual_pct

FROM raw.receipts r FINAL
JOIN raw.blocks b FINAL ON b.block_number = r.block_number
CROSS JOIN window
WHERE r.block_number BETWEEN window.start_block AND window.end_block
  AND r.effective_gas_price > b.base_fee_per_gas
FORMAT Vertical;

-- Identifies where tips go by tracing eth_getBalance changes for fee-recipient
-- precompile accounts.  This query should be run AFTER collecting balance
-- snapshots via the follower service (raw.balance_snapshots).
--
-- The ArbOwnerPublic precompile (0x000000000000000000000000000000000000006b)
-- exposes GetNetworkFeeAccount() and GetInfraFeeAccount().
-- Call these via eth_call before running this query and fill the addresses below.

-- Replace these with real addresses from eth_call results:
WITH
    network_fee_acct AS (SELECT '' AS addr),   -- fill: ArbOwnerPublic.GetNetworkFeeAccount()
    infra_fee_acct   AS (SELECT '' AS addr)    -- fill: ArbOwnerPublic.GetInfraFeeAccount()

-- Balance growth of the network fee account over time (should track priority fees)
SELECT
    s.block_number,
    s.block_timestamp,
    toString(s.balance_wei)                                         AS balance_wei,
    toString(s.balance_wei - lagInFrame(s.balance_wei) OVER
        (ORDER BY s.block_number))                                  AS delta_wei
FROM raw.balance_snapshots s FINAL
CROSS JOIN network_fee_acct
WHERE lower(s.address) = lower(network_fee_acct.addr)
  AND s.block_number >= 0  -- replace with PGA_ACTIVATION_BLOCK
ORDER BY s.block_number
LIMIT 1000
FORMAT Vertical;

-- Cross-check: total tips computed from receipts vs. observed balance growth
-- (should match within rounding; residual = base_fee L2 component also in network_fee_acct)
SELECT
    sum(
        toFloat64(r.effective_gas_price - b.base_fee_per_gas)
        * (r.gas_used - r.gas_used_for_l1)
    ) / 1e18                                                         AS computed_tip_eth,
    'see balance delta above for observed'                           AS observed_tip_eth
FROM raw.receipts r FINAL
JOIN raw.blocks   b FINAL ON b.block_number = r.block_number
WHERE r.block_number BETWEEN 0 AND 1000  -- replace with activation_block and activation_block+1000
;

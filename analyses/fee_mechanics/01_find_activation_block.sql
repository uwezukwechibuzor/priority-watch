-- Finds the PGA activation block: the first block where type-2 txs pay
-- effective_gas_price > base_fee, indicating non-zero priority fees.
-- Run this query after backfilling blocks around 2026-09-24.
-- Save the result as PGA_ACTIVATION_BLOCK in .env.

-- Method A: First type-2 tx with non-zero priority fee in receipt
SELECT
    r.block_number,
    b.block_timestamp,
    toString(b.base_fee_per_gas)                              AS base_fee_wei,
    toString(r.effective_gas_price)                           AS effective_gas_price_wei,
    toString(r.effective_gas_price - b.base_fee_per_gas)      AS priority_fee_wei
FROM raw.receipts r
FINAL
JOIN raw.transactions t FINAL ON r.tx_hash = t.tx_hash
JOIN raw.blocks b FINAL ON r.block_number = b.block_number
WHERE t.tx_type = 2
  AND r.effective_gas_price > b.base_fee_per_gas
  AND b.block_timestamp >= toDateTime('2026-09-20 00:00:00')
ORDER BY r.block_number ASC
LIMIT 1
FORMAT Vertical;

-- Method B: Cross-check — last AuctionResolved round before the candidate block
-- (AuctionResolved rounds stop when Timeboost ends; PGA starts the next block)
SELECT
    toUInt64(reinterpretAsUInt64(reverse(unhex(substring(data, 3, 16))))) AS round,
    block_number,
    block_timestamp
FROM raw.logs
FINAL
WHERE topic0 = '0x' -- replace with TopicTimeboostAuctionResolved hex after deploying
  AND block_timestamp BETWEEN toDateTime('2026-09-01 00:00:00') AND toDateTime('2026-10-01 00:00:00')
ORDER BY block_number DESC
LIMIT 5
FORMAT Vertical;

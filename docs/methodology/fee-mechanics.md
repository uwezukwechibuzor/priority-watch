# Fee Mechanics Methodology

This document describes how Priority Watch computes, verifies, and reconciles
fee data for Arbitrum One's Priority Gas Auction (PGA) ordering policy.

---

## 1. PGA Ordering Mechanics

PGA replaced Timeboost on approximately 2026-09-24 (exact activation block is
determined by the analysis in `analyses/fee_mechanics/01_find_activation_block.sql`).

Under PGA, transactions are ordered within each 125 ms round by descending
**priority fee per gas**:

```
priority_fee_per_gas = min(maxPriorityFeePerGas, maxFeePerGas − baseFee)
```

K = 2 rounds fit per 250 ms block. Ties within a round are broken by arrival
time (not observable on-chain — see §7 Limitations).

An **anti-starvation** mechanism applies a virtual priority boost of `p / (2K)`
to transactions waiting with zero or low tips. This boost is used for ordering
only — it is never charged and never appears in receipt data.

---

## 2. How to Find the Activation Block

### Method A — First non-zero priority fee

The first block where a type-2 transaction has `effective_gas_price > base_fee`
marks the start of PGA.

Query: `analyses/fee_mechanics/01_find_activation_block.sql` (Method A).

This scans `raw.receipts` joined with `raw.blocks` for blocks around 2026-09-24
and returns the earliest such block.

### Method B — Last AuctionResolved event

Timeboost emits `AuctionResolved` events each round. The last such event before
the candidate block from Method A cross-confirms the transition boundary.

Query: `analyses/fee_mechanics/01_find_activation_block.sql` (Method B).

Fill in `topic0` with `keccak256("AuctionResolved(...)")` once the ABI is
confirmed on-chain.

---

## 3. EIP-1559 Formula Verification

For post-activation type-2 transactions the canonical formula is:

```
priority_fee_per_gas = min(maxPriorityFeePerGas, maxFeePerGas − baseFee)
effective_gas_price  = baseFee + priority_fee_per_gas
total_fee_wei        = effective_gas_price × gas_used
```

Query `analyses/fee_mechanics/02_verify_eip1559.sql` samples up to 1,000,000
post-activation type-2 transactions and computes:

- `computed_effective = baseFee + min(maxTip, maxFee − baseFee)`
- Compares against the `effective_gas_price` field in `raw.receipts`

Expected match rate: **100%** (formula is deterministic from the transaction
fields). Any mismatch indicates either an ingestion bug or a non-standard
transaction type.

**Result placeholder**: match_pct = TBD (run after backfilling activation window).

---

## 4. Where Tips Go

Arbitrum One routes fees via two ArbOS precompile accounts:

| Account | Precompile address | Purpose |
|---------|-------------------|---------|
| `infraFeeAccount` | `ArbOwnerPublic.GetInfraFeeAccount()` | L1 calldata cost (gas_used_for_l1 × baseFee) |
| `networkFeeAccount` | `ArbOwnerPublic.GetNetworkFeeAccount()` | L2 base fee + priority tips |

The `ArbOwnerPublic` precompile lives at
`0x000000000000000000000000000000000000006b`.

The `networkFeeAccount` accumulates both:
- **L2 base fee** component: `baseFee × (gas_used − gas_used_for_l1)`
- **Priority tips**: `priority_fee_per_gas × (gas_used − gas_used_for_l1)`  (under H1)

The DAO treasury receives the priority tips through a governance-controlled
distribution (separate from the automatic per-block routing):

- **97%** → Arbitrum DAO Treasury
- **3%** → Arbitrum Developer Guild

Distribution occurs by governance vote every 6 months.

Source: https://forum.arbitrum.foundation/t/constitutional-aip-transition-arbitrum-one-ordering-policy-to-priority-gas-auctions-pga/30942

---

## 5. Which Gas Units Carry the Tip

Arbitrum transactions report two gas quantities:

| Field | Meaning |
|-------|---------|
| `gas_used` | Total gas consumed (L2 + L1 calldata component) |
| `gas_used_for_l1` | Subset of gas_used attributed to L1 calldata cost |

Two hypotheses are tested in `analyses/fee_mechanics/04_reconcile_balance.sql`:

**H1 (working assumption)**: The priority tip is charged only on the L2 gas
component:
```
tip_wei = priority_fee_per_gas × (gas_used − gas_used_for_l1)
```

**H2**: The priority tip is charged on all gas:
```
tip_wei = priority_fee_per_gas × gas_used
```

The reconciliation compares both computed values against the observed balance
delta in `raw.balance_snapshots` for the `networkFeeAccount` over a
1,000-block window.

H1 is expected to match more closely based on Arbitrum's documented fee
routing (the L1 calldata cost goes to `infraFeeAccount` at `baseFee` rate
only, not at the user's tip rate).

---

## 6. Canonical Formula

**Pending reconciliation** — update this section after running
`analyses/fee_mechanics/04_reconcile_balance.sql` with real balance snapshot data.

Working canonical formula (H1 assumed):

```
l1_fee  = baseFee × gas_used_for_l1
                    → infraFeeAccount

l2_base = baseFee × (gas_used − gas_used_for_l1)
                    → networkFeeAccount

tip     = priority_fee_per_gas × (gas_used − gas_used_for_l1)
                    → networkFeeAccount → DAO (97% Treasury / 3% Developer Guild)

total_tx_fee = l1_fee + l2_base + tip
             = effective_gas_price × gas_used
```

---

## 7. Known Limitations

1. **Private mempool**: Arbitrum One uses a private sequencer mempool. Transaction
   arrival times and round boundaries are not observable on-chain. Priority Watch
   cannot reconstruct which PGA round a transaction was placed in, or whether
   anti-starvation boosts were applied.

2. **Round boundary not on-chain**: The 125 ms PGA round boundaries are enforced
   by the sequencer internally. Block timestamps have 250 ms granularity (K=2
   rounds per block), so per-round analysis is an approximation.

3. **`gas_used_for_l1` variance**: The L1 calldata cost component varies with
   Ethereum base fee and calldata size. This affects the split between
   `infraFeeAccount` and `networkFeeAccount` but not the total fee paid.

4. **`effective_gas_price` precision**: Stored as `UInt256` (wei) in ClickHouse
   and as a decimal string in JSON. Floating-point aggregates in SQL may lose
   sub-wei precision at high values; use integer arithmetic where possible.

5. **Treasury distribution lag**: The 97%/3% split is enforced by governance
   vote every 6 months, not block-by-block. The `networkFeeAccount` balance
   accumulates continuously; the actual Treasury transfer happens at a later
   date.

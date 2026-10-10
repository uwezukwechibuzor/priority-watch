# Raw Schema Reference

All tables live in the `raw` database (ClickHouse).  Engine: `ReplacingMergeTree(_version)`.
`_version` is a monotonic UInt64 set by the ingester so re-ingesting a block is idempotent.
`_ingested_at` defaults to `now64(3)` and is never queried for analytics.

---

## raw.blocks

One row per Arbitrum One block.  Partitioned by `toYYYYMM(block_timestamp)`, ordered by `block_number`.

| Column | Type | Notes |
|---|---|---|
| block_number | UInt64 | Primary sort key |
| block_hash | FixedString(66) | `0x`-prefixed 32-byte hex |
| parent_hash | FixedString(66) | |
| block_timestamp | DateTime64(3,'UTC') | Millisecond-precision Unix epoch |
| l1_block_number | UInt64 | L1 block at sequencing time |
| base_fee_per_gas | UInt256 | Wei |
| gas_used | UInt64 | |
| gas_limit | UInt64 | |
| tx_count | UInt32 | |
| size | UInt32 | Bytes |
| _ingested_at | DateTime64(3,'UTC') | DEFAULT now64(3) |
| _version | UInt64 | Upsert version |

Skip indexes: `bloom_filter` on `block_hash`, `parent_hash`.

---

## raw.transactions

One row per transaction.  Partitioned by `toYYYYMM(block_timestamp)`, ordered by `(block_number, tx_index)`.

| Column | Type | Notes |
|---|---|---|
| block_number | UInt64 | |
| block_hash | FixedString(66) | |
| block_timestamp | DateTime64(3,'UTC') | Denormalised from block |
| tx_hash | FixedString(66) | |
| tx_index | UInt32 | Position in block |
| from_address | FixedString(42) | `0x`-prefixed |
| to_address | FixedString(42) | Empty string for contract creation |
| value | UInt256 | Wei transferred |
| gas | UInt64 | Gas limit |
| gas_price | UInt256 | Effective gas price in wei |
| max_fee_per_gas | Nullable(UInt256) | EIP-1559; NULL for legacy txs |
| max_priority_fee_per_gas | Nullable(UInt256) | EIP-1559; NULL for legacy txs |
| priority_fee_per_gas | UInt256 | `min(maxPriorityFeePerGas, maxFeePerGas − baseFee)` |
| tx_type | UInt8 | 0=legacy, 2=EIP-1559 |
| nonce | UInt64 | |
| input_size | UInt32 | Bytes of calldata |
| _ingested_at | DateTime64(3,'UTC') | DEFAULT now64(3) |
| _version | UInt64 | |

Skip indexes: `bloom_filter` on `tx_hash`, `from_address`, `to_address`.

---

## raw.receipts

One row per transaction receipt.  Partitioned by `toYYYYMM(block_timestamp)`, ordered by `(block_number, tx_index)`.

| Column | Type | Notes |
|---|---|---|
| block_number | UInt64 | |
| block_hash | FixedString(66) | |
| block_timestamp | DateTime64(3,'UTC') | Denormalised for partitioning |
| tx_hash | FixedString(66) | |
| tx_index | UInt32 | |
| from_address | FixedString(42) | |
| to_address | FixedString(42) | |
| contract_address | FixedString(42) | Empty unless contract deployment |
| status | UInt8 | 1=success, 0=reverted |
| gas_used | UInt64 | Actual gas consumed |
| cumulative_gas_used | UInt64 | |
| effective_gas_price | UInt256 | Wei |
| logs_count | UInt32 | Number of emitted logs |
| _ingested_at | DateTime64(3,'UTC') | DEFAULT now64(3) |
| _version | UInt64 | |

Skip index: `bloom_filter` on `tx_hash`.

---

## raw.logs

One row per event log.  Partitioned by `toYYYYMM(block_timestamp)`, ordered by `(block_number, tx_index, log_index)`.

| Column | Type | Notes |
|---|---|---|
| block_number | UInt64 | |
| block_hash | FixedString(66) | |
| block_timestamp | DateTime64(3,'UTC') | Denormalised |
| tx_hash | FixedString(66) | |
| tx_index | UInt32 | |
| log_index | UInt32 | Position within block |
| address | FixedString(42) | Emitting contract |
| topic0 | FixedString(66) | Event signature hash |
| topic1 | FixedString(66) | Indexed param 1 |
| topic2 | FixedString(66) | Indexed param 2 |
| topic3 | FixedString(66) | Indexed param 3 |
| data | String | ABI-encoded non-indexed params |
| data_size | UInt32 | Byte length of `data` |
| _ingested_at | DateTime64(3,'UTC') | DEFAULT now64(3) |
| _version | UInt64 | |

Skip indexes: `bloom_filter` on `tx_hash`, `address`, `topic0`.

---

## raw.cex_klines_1s

One-second OHLCV klines from Binance spot (ETH/USDT).  Used to align gas-price spikes with ETH price events.

| Column | Type | Notes |
|---|---|---|
| ts | DateTime64(3,'UTC') | Open time, primary sort key |
| symbol | LowCardinality(String) | e.g. `ETHUSDT` |
| open | Decimal(18,8) | |
| high | Decimal(18,8) | |
| low | Decimal(18,8) | |
| close | Decimal(18,8) | |
| volume | Decimal(26,8) | Base asset volume |
| quote_volume | Decimal(26,8) | Quote asset volume |
| trade_count | UInt32 | |
| _ingested_at | DateTime64(3,'UTC') | DEFAULT now64(3) |
| _version | UInt64 | |

Partitioned by `toYYYYMM(ts)`.

---

## raw.balance_snapshots

Periodic snapshots of EOA/contract ETH balances sampled once per block (task 4+ scope).

| Column | Type | Notes |
|---|---|---|
| block_number | UInt64 | |
| address | FixedString(42) | |
| balance | UInt256 | Wei |
| _ingested_at | DateTime64(3,'UTC') | DEFAULT now64(3) |
| _version | UInt64 | |

Partitioned by `toYYYYMM(toDateTime(block_number * 0.25))` (approximate; refined in task 5).
Ordered by `(block_number, address)`.

---

## raw.timeboost_bids_ext

External baseline: Timeboost express-lane bid data imported from timeboost.app Parquet exports.
Enables direct PGA vs. Timeboost comparison in `marts`.

| Column | Type | Notes |
|---|---|---|
| round_number | UInt64 | Timeboost 250 ms auction round |
| bidder | FixedString(42) | Bidder address |
| amount_wei | UInt256 | Bid in wei |
| express_lane_controller | FixedString(42) | Address that would receive the slot |
| auction_contract | FixedString(42) | |
| chain_id | UInt64 | |
| block_number | UInt64 | Block where bid was resolved |
| block_timestamp | DateTime64(3,'UTC') | |
| _ingested_at | DateTime64(3,'UTC') | DEFAULT now64(3) |
| _version | UInt64 | |

Partitioned by `toYYYYMM(block_timestamp)`, ordered by `(round_number, bidder)`.
Skip index: `bloom_filter` on `bidder`.

---

## raw.reorg_log

Tracks L2 chain reorganisations detected during ingestion.

| Column | Type | Notes |
|---|---|---|
| detected_at | DateTime64(3,'UTC') | When the reorg was observed |
| canonical_block_number | UInt64 | Last canonical block before the reorg |
| canonical_block_hash | FixedString(66) | |
| orphan_block_number | UInt64 | First orphaned block |
| orphan_block_hash | FixedString(66) | |
| depth | UInt32 | Number of orphaned blocks |
| _ingested_at | DateTime64(3,'UTC') | DEFAULT now64(3) |

Ordered by `detected_at`.  No partitioning (reorgs are rare).

---

## Meta tables

Managed by the migration runner in the `meta` database.

| Table | Purpose |
|---|---|
| meta.schema_migrations | Tracks applied migration files by SHA-256 checksum |
| meta.ingest_cursors | Per-source ingestion state (last block, status) |
| meta.source_status | Per-source freshness snapshot for the `/health/sources` API |

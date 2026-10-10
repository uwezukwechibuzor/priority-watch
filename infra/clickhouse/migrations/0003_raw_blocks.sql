-- raw.blocks: one row per Arbitrum One block.
-- base_fee_per_gas stored as UInt256 (wei); all integer wei fields use UInt256.
-- Deduplication key: block_number (ORDER BY); _version allows idempotent re-ingestion.
-- Bloom filters on block_hash and parent_hash for fast point lookups.
CREATE TABLE IF NOT EXISTS raw.blocks
(
    block_number     UInt64           CODEC(Delta, ZSTD(1)),
    block_hash       FixedString(66)  CODEC(ZSTD(1)),
    parent_hash      FixedString(66)  CODEC(ZSTD(1)),
    block_timestamp  DateTime64(3, 'UTC') CODEC(Delta, ZSTD(1)),
    l1_block_number  UInt64           CODEC(Delta, ZSTD(1)),
    base_fee_per_gas UInt256          CODEC(ZSTD(1)),
    gas_used         UInt64           CODEC(ZSTD(1)),
    gas_limit        UInt64           CODEC(ZSTD(1)),
    tx_count         UInt32           CODEC(ZSTD(1)),
    size             UInt32           CODEC(ZSTD(1)),
    _ingested_at     DateTime64(3, 'UTC') DEFAULT now64(3) CODEC(ZSTD(1)),
    _version         UInt64           CODEC(ZSTD(1)),

    INDEX idx_block_hash  block_hash  TYPE bloom_filter GRANULARITY 4,
    INDEX idx_parent_hash parent_hash TYPE bloom_filter GRANULARITY 4
)
ENGINE = ReplacingMergeTree(_version)
PARTITION BY toYYYYMM(block_timestamp)
ORDER BY block_number
SETTINGS index_granularity = 8192

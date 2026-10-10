-- raw.balance_snapshots: hourly ETH balance snapshots for fee-recipient addresses.
-- Used to verify DAO treasury accumulation matches PGA tip revenue.
-- balance_wei in UInt256 (wei, never floating point).
CREATE TABLE IF NOT EXISTS raw.balance_snapshots
(
    address         FixedString(42)      CODEC(ZSTD(1)),
    block_number    UInt64               CODEC(Delta, ZSTD(1)),
    block_timestamp DateTime64(3, 'UTC') CODEC(Delta, ZSTD(1)),
    balance_wei     UInt256              CODEC(ZSTD(1)),
    _ingested_at    DateTime64(3, 'UTC') DEFAULT now64(3) CODEC(ZSTD(1)),
    _version        UInt64               CODEC(ZSTD(1)),

    INDEX idx_address address TYPE bloom_filter GRANULARITY 4
)
ENGINE = ReplacingMergeTree(_version)
PARTITION BY toYYYYMM(block_timestamp)
ORDER BY (address, block_number)
SETTINGS index_granularity = 8192

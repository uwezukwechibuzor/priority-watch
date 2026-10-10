-- raw.logs: decoded event logs for configured contract addresses only.
-- The ingester filters by address using config/contracts.yaml at ingestion time.
-- topic1-3 are empty string when the event has fewer than 4 indexed parameters.
-- data is hex-encoded raw log data (ABI-decoded fields are in intermediate models).
CREATE TABLE IF NOT EXISTS raw.logs
(
    block_number    UInt64           CODEC(Delta, ZSTD(1)),
    block_timestamp DateTime64(3, 'UTC') CODEC(Delta, ZSTD(1)),
    tx_index        UInt32           CODEC(Delta, ZSTD(1)),
    tx_hash         FixedString(66)  CODEC(ZSTD(1)),
    log_index       UInt32           CODEC(Delta, ZSTD(1)),
    address         FixedString(42)  CODEC(ZSTD(1)),
    topic0          FixedString(66)  CODEC(ZSTD(1)),
    topic1          String           CODEC(ZSTD(1)),
    topic2          String           CODEC(ZSTD(1)),
    topic3          String           CODEC(ZSTD(1)),
    data            String           CODEC(ZSTD(1)),
    _ingested_at    DateTime64(3, 'UTC') DEFAULT now64(3) CODEC(ZSTD(1)),
    _version        UInt64           CODEC(ZSTD(1)),

    INDEX idx_tx_hash tx_hash  TYPE bloom_filter GRANULARITY 4,
    INDEX idx_address address  TYPE bloom_filter GRANULARITY 4,
    INDEX idx_topic0  topic0   TYPE bloom_filter GRANULARITY 4
)
ENGINE = ReplacingMergeTree(_version)
PARTITION BY toYYYYMM(block_timestamp)
ORDER BY (block_number, log_index)
SETTINGS index_granularity = 8192

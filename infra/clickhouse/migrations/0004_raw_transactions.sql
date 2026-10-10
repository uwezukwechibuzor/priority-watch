-- raw.transactions: one row per transaction in every block.
-- tx_type UInt8 covers EIP-2718 types 0x00-0x02 and Arbitrum system types 0x64-0x6a.
-- max_priority_fee_per_gas NULL for non-EIP-1559 transactions (type 0 and 1).
-- input_selector: first 4 bytes of calldata as hex (0x + 8 chars); empty if calldata < 4 bytes.
-- from_address / to_address: lowercase 0x-prefixed hex; to_address empty for contract creation.
CREATE TABLE IF NOT EXISTS raw.transactions
(
    block_number             UInt64           CODEC(Delta, ZSTD(1)),
    block_timestamp          DateTime64(3, 'UTC') CODEC(Delta, ZSTD(1)),
    tx_index                 UInt32           CODEC(Delta, ZSTD(1)),
    tx_hash                  FixedString(66)  CODEC(ZSTD(1)),
    tx_type                  UInt8            CODEC(ZSTD(1)),
    from_address             String           CODEC(ZSTD(1)),
    to_address               String           CODEC(ZSTD(1)),
    nonce                    UInt64           CODEC(ZSTD(1)),
    value                    UInt256          CODEC(ZSTD(1)),
    gas_limit                UInt64           CODEC(ZSTD(1)),
    gas_price                UInt256          CODEC(ZSTD(1)),
    max_fee_per_gas          Nullable(UInt256),
    max_priority_fee_per_gas Nullable(UInt256),
    input_selector           String           CODEC(ZSTD(1)),
    input_size               UInt32           CODEC(ZSTD(1)),
    _ingested_at             DateTime64(3, 'UTC') DEFAULT now64(3) CODEC(ZSTD(1)),
    _version                 UInt64           CODEC(ZSTD(1)),

    INDEX idx_tx_hash      tx_hash       TYPE bloom_filter GRANULARITY 4,
    INDEX idx_from_address from_address  TYPE bloom_filter GRANULARITY 4,
    INDEX idx_to_address   to_address    TYPE bloom_filter GRANULARITY 4,
    INDEX idx_selector     input_selector TYPE bloom_filter GRANULARITY 4
)
ENGINE = ReplacingMergeTree(_version)
PARTITION BY toYYYYMM(block_timestamp)
ORDER BY (block_number, tx_index)
SETTINGS index_granularity = 8192

-- raw.receipts: transaction execution results.
-- block_timestamp is denormalised from raw.blocks for monthly partitioning.
-- gas_used_for_l1: Arbitrum-specific L1 calldata fee portion (Nitro ArbGasInfo).
-- effective_gas_price: actual wei/gas paid (min(max_fee, base_fee + max_priority_fee)).
-- contract_address: empty string unless this tx deployed a contract.
CREATE TABLE IF NOT EXISTS raw.receipts
(
    block_number        UInt64           CODEC(Delta, ZSTD(1)),
    block_timestamp     DateTime64(3, 'UTC') CODEC(Delta, ZSTD(1)),
    tx_index            UInt32           CODEC(Delta, ZSTD(1)),
    tx_hash             FixedString(66)  CODEC(ZSTD(1)),
    status              UInt8            CODEC(ZSTD(1)),
    gas_used            UInt64           CODEC(ZSTD(1)),
    gas_used_for_l1     UInt64           CODEC(ZSTD(1)),
    effective_gas_price UInt256          CODEC(ZSTD(1)),
    cumulative_gas_used UInt64           CODEC(ZSTD(1)),
    contract_address    String           CODEC(ZSTD(1)),
    log_count           UInt32           CODEC(ZSTD(1)),
    _ingested_at        DateTime64(3, 'UTC') DEFAULT now64(3) CODEC(ZSTD(1)),
    _version            UInt64           CODEC(ZSTD(1)),

    INDEX idx_tx_hash       tx_hash          TYPE bloom_filter GRANULARITY 4,
    INDEX idx_contract_addr contract_address TYPE bloom_filter GRANULARITY 4
)
ENGINE = ReplacingMergeTree(_version)
PARTITION BY toYYYYMM(block_timestamp)
ORDER BY (block_number, tx_index)
SETTINGS index_granularity = 8192

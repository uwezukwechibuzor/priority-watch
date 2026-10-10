-- raw.reorg_log: records detected chain reorganisations for operational monitoring.
-- Append-only; no ReplacingMergeTree since each reorg event is unique.
-- depth: number of blocks rolled back.
CREATE TABLE IF NOT EXISTS raw.reorg_log
(
    detected_at DateTime64(3, 'UTC') CODEC(Delta, ZSTD(1)),
    fork_block  UInt64               CODEC(ZSTD(1)),
    old_hash    FixedString(66)      CODEC(ZSTD(1)),
    new_hash    FixedString(66)      CODEC(ZSTD(1)),
    depth       UInt32               CODEC(ZSTD(1))
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(detected_at)
ORDER BY (detected_at, fork_block)
SETTINGS index_granularity = 1024

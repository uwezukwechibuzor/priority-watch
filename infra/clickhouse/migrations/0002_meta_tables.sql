-- meta.ingest_cursors tracks ingestion job state (range, status, last seen block).
CREATE TABLE IF NOT EXISTS meta.ingest_cursors
(
    job         LowCardinality(String)       CODEC(ZSTD(1)),
    range_start UInt64                       CODEC(ZSTD(1)),
    range_end   UInt64                       CODEC(ZSTD(1)),
    status      LowCardinality(String)       CODEC(ZSTD(1)),
    last_block  UInt64                       CODEC(ZSTD(1)),
    attempts    UInt32                       CODEC(ZSTD(1)),
    error       String                       CODEC(ZSTD(1)),
    updated_at  DateTime64(3, 'UTC')         CODEC(ZSTD(1))
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY job
SETTINGS index_granularity = 1024;

-- meta.source_status feeds the per-source freshness endpoint (mirrors timeboost.app).
CREATE TABLE IF NOT EXISTS meta.source_status
(
    source           LowCardinality(String)  CODEC(ZSTD(1)),
    rows             UInt64                  CODEC(ZSTD(1)),
    latest_position  String                  CODEC(ZSTD(1)),
    latest_timestamp DateTime64(3, 'UTC')    CODEC(ZSTD(1)),
    updated_at       DateTime64(3, 'UTC')    CODEC(ZSTD(1))
)
ENGINE = ReplacingMergeTree(updated_at)
ORDER BY source
SETTINGS index_granularity = 1024

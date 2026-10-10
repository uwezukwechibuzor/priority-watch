-- raw.cex_klines_1s: Binance 1-second OHLCV klines for ETH/USD price conversion.
-- venue is always 'binance' currently; kept as LowCardinality for potential future sources.
-- open/high/low/close in USD (Float64 is sufficient precision for display and conversion).
CREATE TABLE IF NOT EXISTS raw.cex_klines_1s
(
    venue     LowCardinality(String)  CODEC(ZSTD(1)),
    symbol    LowCardinality(String)  CODEC(ZSTD(1)),
    open_time DateTime64(3, 'UTC')    CODEC(Delta, ZSTD(1)),
    open      Float64                 CODEC(ZSTD(1)),
    high      Float64                 CODEC(ZSTD(1)),
    low       Float64                 CODEC(ZSTD(1)),
    close     Float64                 CODEC(ZSTD(1)),
    volume    Float64                 CODEC(ZSTD(1)),
    trades    UInt32                  CODEC(ZSTD(1)),
    _ingested_at DateTime64(3, 'UTC') DEFAULT now64(3) CODEC(ZSTD(1)),
    _version  UInt64                  CODEC(ZSTD(1))
)
ENGINE = ReplacingMergeTree(_version)
PARTITION BY toYYYYMM(open_time)
ORDER BY (venue, symbol, open_time)
SETTINGS index_granularity = 8192

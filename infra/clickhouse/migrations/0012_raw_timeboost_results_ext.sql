-- raw.timeboost_auction_results_ext: Timeboost auction results imported as the pre-PGA baseline.
-- Contains per-round auction outcomes from timeboost.app public export.
-- Keyed by round; winner and express_lane_controller are the addresses that won the round.
-- first_price_wei / second_price_wei are the top two bids placed in that round.
-- Used by /compare to contrast Timeboost auction outcomes with PGA tip revenue.
CREATE TABLE IF NOT EXISTS raw.timeboost_auction_results_ext
(
    round                    UInt64           CODEC(Delta, ZSTD(1)),
    winner                   FixedString(42)  CODEC(ZSTD(1)),
    express_lane_controller  FixedString(42)  CODEC(ZSTD(1)),
    first_price_wei          UInt256          CODEC(ZSTD(1)),
    second_price_wei         UInt256          CODEC(ZSTD(1)),
    round_start              DateTime64(3,'UTC') CODEC(Delta, ZSTD(1)),
    round_end                DateTime64(3,'UTC') CODEC(Delta, ZSTD(1)),
    source                   LowCardinality(String) CODEC(ZSTD(1)),
    _ingested_at             DateTime64(3,'UTC') DEFAULT now64(3) CODEC(ZSTD(1)),
    _version                 UInt64           CODEC(ZSTD(1))
)
ENGINE = ReplacingMergeTree(_version)
ORDER BY round
SETTINGS index_granularity = 4096

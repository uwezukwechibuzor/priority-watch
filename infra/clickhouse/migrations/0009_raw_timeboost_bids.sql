-- raw.timeboost_bids_ext: Timeboost auction bids imported as the pre-PGA baseline.
-- Used by /compare to contrast Timeboost revenue with PGA tip revenue.
-- Source is the timeboost.app Parquet export or direct Timeboost contract event logs.
-- amount_wei: bid amount in wei (UInt256 for full precision).
-- express_lane_controller: the address that would receive express-lane rights if this bid won.
CREATE TABLE IF NOT EXISTS raw.timeboost_bids_ext
(
    round                    UInt64           CODEC(Delta, ZSTD(1)),
    bidder                   FixedString(42)  CODEC(ZSTD(1)),
    express_lane_controller  FixedString(42)  CODEC(ZSTD(1)),
    amount_wei               UInt256          CODEC(ZSTD(1)),
    bid_timestamp            DateTime64(3, 'UTC') CODEC(Delta, ZSTD(1)),
    source                   LowCardinality(String) CODEC(ZSTD(1)),
    _ingested_at             DateTime64(3, 'UTC') DEFAULT now64(3) CODEC(ZSTD(1)),
    _version                 UInt64           CODEC(ZSTD(1)),

    INDEX idx_bidder                  bidder                 TYPE bloom_filter GRANULARITY 4,
    INDEX idx_express_lane_controller express_lane_controller TYPE bloom_filter GRANULARITY 4
)
ENGINE = ReplacingMergeTree(_version)
PARTITION BY toYYYYMM(bid_timestamp)
ORDER BY (round, bidder)
SETTINGS index_granularity = 8192

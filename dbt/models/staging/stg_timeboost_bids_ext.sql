{{
    config(
        order_by = '(round, bidder)',
        partition_by = 'toYYYYMM(bid_timestamp)'
    )
}}

-- Typed and normalised Timeboost auction bids from the external import.
-- Source data comes from timeboost.app public export (Apache-2.0), loaded by
-- `pw-ingester import-timeboost`.  Covers the Timeboost era 2025-04-17 – ~2026-09-24.
-- Used by /compare to contrast Timeboost auction revenue with PGA tip revenue.
SELECT
    round,
    lower(bidder)                       AS bidder,
    lower(express_lane_controller)      AS express_lane_controller,
    amount_wei,
    {{ wei_to_eth('amount_wei') }}      AS amount_eth,
    bid_timestamp,
    source,
    _ingested_at
FROM {{ source('raw', 'timeboost_bids_ext') }} FINAL
{% if is_incremental() %}
WHERE bid_timestamp > (
    SELECT subtractHours(max(bid_timestamp), 1)
    FROM {{ this }} FINAL
)
{% endif %}

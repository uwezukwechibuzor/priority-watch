{{
    config(
        order_by = 'round',
        partition_by = 'toYYYYMM(toDateTime(round * 60))'
    )
}}

-- Per-round Timeboost auction statistics from the external bids import.
-- This model provides the baseline for the /compare dashboard:
-- Timeboost auction clearing prices vs PGA tip revenue per era.
--
-- round is the Timeboost round number (sequential integer from timeboost.app export).
-- clearing_price_wei: second-highest bid (Vickrey auction — winner pays 2nd price).
-- If only one bid exists, clearing_price_wei = that bid (no second price).
--
-- winner_entity comes from the entities seed via LEFT JOIN; NULL if unlabelled.
WITH bids_by_round AS (
    SELECT
        round,
        -- Sorted bids descending for 1st/2nd extraction
        arraySort(
            x -> -toFloat64(x),
            groupUniqArray(amount_wei)
        )                                                   AS bids_sorted,
        count()                                             AS bid_count,
        min(bid_timestamp)                                  AS round_start_ts,
        max(bid_timestamp)                                  AS round_end_ts
    FROM {{ ref('stg_timeboost_bids_ext') }} FINAL
    GROUP BY round
),

round_results AS (
    SELECT
        r.round,
        r.winner                                AS winner_address,
        lower(r.winner)                         AS winner_address_lower
    FROM {{ source('raw', 'timeboost_auction_results_ext') }} AS r FINAL
)

SELECT
    b.round                                                             AS round,
    b.round_start_ts,
    b.round_end_ts,
    b.bid_count,
    -- First bid (highest)
    if(length(b.bids_sorted) >= 1, b.bids_sorted[1], toUInt256(0))     AS first_bid_wei,
    -- Clearing price = second-highest bid (Vickrey); falls back to first if only one bid
    if(
        length(b.bids_sorted) >= 2,
        b.bids_sorted[2],
        if(length(b.bids_sorted) = 1, b.bids_sorted[1], toUInt256(0))
    )                                                                   AS clearing_price_wei,
    -- 2nd/1st ratio (contestedness proxy; 1.0 = perfectly competitive)
    if(
        length(b.bids_sorted) >= 2 AND b.bids_sorted[1] > 0,
        toFloat64(b.bids_sorted[2]) / toFloat64(b.bids_sorted[1]),
        toFloat64(0)
    )                                                                   AS second_to_first_ratio,
    {{ wei_to_eth('if(length(b.bids_sorted) >= 1, b.bids_sorted[1], toUInt256(0))') }}        AS first_bid_eth,
    {{ wei_to_eth('if(length(b.bids_sorted) >= 2, b.bids_sorted[2], if(length(b.bids_sorted) = 1, b.bids_sorted[1], toUInt256(0)))') }}  AS clearing_price_eth,
    rr.winner_address                                                   AS winner_address,
    e.entity_name                                                       AS winner_entity,
    e.entity_type                                                       AS winner_entity_type
FROM bids_by_round b
LEFT JOIN round_results rr ON b.round = rr.round
LEFT JOIN {{ ref('entities') }} e
    ON rr.winner_address_lower = lower(e.address)
    AND e.address != ''

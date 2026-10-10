{{
    config(
        order_by = '(block_number, log_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- On-chain AuctionResolved events from the ExpressLaneAuction contract.
-- The exact topic0 is a placeholder (all zeros) until the contract ABI is confirmed
-- on Arbiscan (see CLAUDE.md §11 and analyses/fee_mechanics/01_find_activation_block.sql).
-- Until timeboost_auction_resolved_topic0 is updated in dbt_project.yml vars,
-- this model returns zero rows.
--
-- ABI decoding is deferred: raw topic/data fields are carried forward for the
-- intermediate layer (task 10) to decode once the ABI is confirmed.
SELECT
    block_number,
    block_timestamp,
    tx_hash,
    log_index,
    lower(address)  AS contract_address,
    lower(topic0)   AS topic0,
    lower(topic1)   AS topic1_raw,
    lower(topic2)   AS topic2_raw,
    lower(topic3)   AS topic3_raw,
    data            AS data_raw,
    _ingested_at
FROM {{ source('raw', 'logs') }} FINAL
WHERE lower(address) = lower('{{ var("express_lane_auction_address") }}')
    AND lower(topic0) = lower('{{ var("timeboost_auction_resolved_topic0") }}')
{% if is_incremental() %}
    AND block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
{% endif %}

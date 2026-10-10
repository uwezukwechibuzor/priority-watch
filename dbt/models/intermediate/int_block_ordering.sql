{{
    config(
        order_by = '(block_number, tx_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Per-transaction ordering metrics with block-level statistics denormalized.
-- NOTE: PGA rounds (125 ms windows) are NOT observable on chain — arrival time and
-- round boundaries are private mempool data. estimated_rounds is an approximation:
-- each new non-increasing run of fees within a block is counted as one round boundary.
-- The Spearman ordering_score (corrStable over tipped txs) measures how closely
-- fee-descending rank matches transaction index; +1.0 = perfect ordering.
WITH tx_fees AS (
    SELECT
        block_number,
        block_timestamp,
        tx_index,
        tx_hash,
        from_address,
        priority_fee_per_gas,
        tip_paid_wei,
        era
    FROM {{ ref('int_tx_fees') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
),

-- Dense-rank by priority fee descending within each block (tipped txs only get rank 1+)
tx_ranked AS (
    SELECT
        block_number,
        block_timestamp,
        tx_index,
        tx_hash,
        from_address,
        priority_fee_per_gas,
        tip_paid_wei,
        era,
        -- tip_rank: 1 = highest fee in block; 0 = untipped (priority_fee = 0)
        if(
            priority_fee_per_gas > 0,
            toUInt32(dense_rank() OVER (
                PARTITION BY block_number
                ORDER BY priority_fee_per_gas DESC
            )),
            toUInt32(0)
        ) AS tip_rank,
        -- prev_fee for run detection (NULL for first tx in block)
        lagInFrame(priority_fee_per_gas, 1, toUInt256(0)) OVER (
            PARTITION BY block_number
            ORDER BY tx_index
        ) AS prev_fee,
        -- is_run_start: this tipped tx has a higher fee than the preceding tipped tx
        if(
            priority_fee_per_gas > 0
            AND lagInFrame(priority_fee_per_gas, 1, toUInt256(0)) OVER (
                PARTITION BY block_number
                ORDER BY tx_index
            ) < priority_fee_per_gas
            AND lagInFrame(priority_fee_per_gas, 1, toUInt256(0)) OVER (
                PARTITION BY block_number
                ORDER BY tx_index
            ) > toUInt256(0),
            toUInt8(1),
            toUInt8(0)
        ) AS is_run_start
    FROM tx_fees
),

-- Spearman correlation and block-level aggregations (over tipped txs only)
block_stats AS (
    SELECT
        block_number,
        -- Spearman ordering score on tipped txs: corrStable(tip_rank, tx_index)
        -- ClickHouse corrStable is Pearson on ranks; identical to Spearman here
        -- because tip_rank and tx_index are integer series.
        if(
            countIf(priority_fee_per_gas > 0) >= 2,
            corrStable(
                toFloat64(tip_rank),
                toFloat64(tx_index)
            ),
            toFloat64(0)
        ) AS ordering_score,
        -- Sorted distinct priority fees (descending) for top/second fee extraction
        arraySort(
            x -> -toFloat64(x),
            groupUniqArrayIf(priority_fee_per_gas, priority_fee_per_gas > 0)
        ) AS tipped_fees,
        countIf(priority_fee_per_gas > 0)                           AS tipped_tx_count,
        uniqIf(from_address, priority_fee_per_gas > 0)              AS distinct_tippers,
        -- estimated_rounds: count of run restarts (fee jumps up from prior tipped tx)
        -- The first tipped tx in a block always opens round 1; subsequent restarts add 1.
        -- This is an estimation only — actual round boundaries are not observable.
        greatest(
            toUInt32(1) * toUInt32(countIf(priority_fee_per_gas > 0) > 0),
            toUInt32(countIf(priority_fee_per_gas > 0 AND is_run_start = 1) + toUInt32(countIf(priority_fee_per_gas > 0) > 0))
        ) AS estimated_rounds
    FROM tx_ranked
    GROUP BY block_number
),

-- Join tx rows with block-level stats
final_rows AS (
    SELECT
        t.block_number,
        t.block_timestamp,
        t.tx_index,
        t.tx_hash,
        t.from_address,
        t.priority_fee_per_gas,
        t.tip_paid_wei,
        t.era,
        t.tip_rank,
        t.prev_fee,
        t.is_run_start,
        s.ordering_score,
        s.tipped_fees,
        s.tipped_tx_count,
        s.distinct_tippers,
        s.estimated_rounds,
        -- Top fee and second fee from sorted array
        if(length(s.tipped_fees) >= 1, s.tipped_fees[1], toUInt256(0))  AS top_fee,
        if(length(s.tipped_fees) >= 2, s.tipped_fees[2], toUInt256(0))  AS second_fee,
        -- 2nd/1st ratio: null when fewer than 2 distinct tipped fee levels
        if(
            length(s.tipped_fees) >= 2 AND s.tipped_fees[1] > 0,
            toFloat64(s.tipped_fees[2]) / toFloat64(s.tipped_fees[1]),
            toFloat64(0)
        ) AS second_to_first_ratio,
        -- block_type classification
        CASE
            WHEN s.distinct_tippers = 0 THEN 'untipped'
            WHEN s.distinct_tippers = 1 THEN 'single'
            ELSE 'contested'
        END AS block_type,
        -- Anomaly flag: more than 4 estimated rounds in a 2-round block is suspicious
        toUInt8(s.estimated_rounds > 4) AS has_anomaly
    FROM tx_ranked t
    INNER JOIN block_stats s ON t.block_number = s.block_number
)

SELECT
    block_number,
    block_timestamp,
    tx_index,
    tx_hash,
    from_address,
    priority_fee_per_gas,
    tip_paid_wei,
    era,
    tip_rank,
    tipped_tx_count,
    distinct_tippers,
    block_type,
    {{ to_gwei('top_fee') }}                                    AS top_fee_gwei,
    {{ to_gwei('second_fee') }}                                 AS second_fee_gwei,
    second_to_first_ratio,
    ordering_score,
    estimated_rounds,
    has_anomaly
FROM final_rows

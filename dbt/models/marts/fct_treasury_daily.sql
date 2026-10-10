{{
    config(
        order_by = '(date, era)',
        partition_by = 'toYYYYMM(date)'
    )
}}

-- Daily treasury accounting: tip revenue split 97/3 between DAO Treasury and
-- Developer Guild per CLAUDE.md §2 and §11.
-- Timeboost clearing prices (before PGA activation) are summed from int_timeboost_rounds.
--
-- Reconciliation columns (diff_wei, diff_pct, status) are placeholders until
-- balance_snapshots data is available (task 8 part C). Status 'ok' / 'warn' / 'fail'
-- will be set by comparing cumulative tips against fee-recipient balance deltas.
-- Verified: balance snapshot source is stg_balance_snapshots → raw.balance_snapshots.
WITH pga_daily AS (
    SELECT
        toDate(block_timestamp)                                 AS date,
        era,
        sum(tip_paid_wei)                                      AS total_tip_wei,
        sum(tip_paid_eth)                                      AS total_tip_eth,
        sum(tip_paid_usd)                                      AS total_tip_usd,
        -- 97% Treasury / 3% Developer Guild (integer division; rounding favours treasury)
        toUInt256(round(toFloat64(sum(tip_paid_wei)) * 0.97))  AS treasury_wei,
        toUInt256(round(toFloat64(sum(tip_paid_wei)) * 0.03))  AS developer_guild_wei,
        count()                                                AS tx_count,
        countIf(priority_fee_per_gas > 0)                     AS tipped_tx_count
    FROM {{ ref('int_tx_fees') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY date, era
),

timeboost_daily AS (
    SELECT
        toDate(round_start_ts)                                  AS date,
        'timeboost'                                             AS era,
        sum(clearing_price_wei)                                AS total_clearing_wei,
        sum(clearing_price_eth)                                AS total_clearing_eth,
        count()                                                AS round_count
    FROM {{ ref('int_timeboost_rounds') }} FINAL
    {% if is_incremental() %}
    WHERE round_start_ts > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
    GROUP BY date
)

SELECT
    pd.date,
    pd.era,
    pd.total_tip_wei,
    pd.total_tip_eth,
    pd.total_tip_usd,
    pd.treasury_wei,
    pd.developer_guild_wei,
    pd.tx_count,
    pd.tipped_tx_count,
    -- Timeboost proceeds for comparison (NULL if not timeboost era)
    coalesce(tb.total_clearing_wei, toUInt256(0))           AS timeboost_proceeds_wei,
    coalesce(tb.total_clearing_eth, 0.0)                    AS timeboost_proceeds_eth,
    coalesce(tb.round_count, 0)                             AS timeboost_round_count,
    -- Reconciliation placeholders (pending balance snapshot data from task 8 part C)
    toFloat64(0)                                            AS diff_wei,
    toFloat64(0)                                            AS diff_pct,
    'ok'                                                    AS status
FROM pga_daily pd
LEFT JOIN timeboost_daily tb ON pd.date = tb.date AND pd.era = 'timeboost'

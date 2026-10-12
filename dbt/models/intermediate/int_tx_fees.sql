{{
    config(
        order_by = '(block_number, tx_index)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- Fee breakdown per transaction using the canonical PGA formula from CLAUDE.md §11.
-- H1 hypothesis: tip is charged on L2 gas only (gas_used_for_l2), not total gas.
-- ASOF JOIN to the nearest preceding Binance 1s kline for ETH/USD conversion.
-- Era classification is deferred to a variable; PGA activation block defaults to 0
-- (all blocks treated as timeboost) until set by task 8 verification.
WITH tx_base AS (
    SELECT
        t.block_number          AS block_number,
        t.block_timestamp       AS block_timestamp,
        t.tx_index              AS tx_index,
        t.tx_hash               AS tx_hash,
        t.tx_type               AS tx_type,
        t.from_address          AS from_address,
        t.to_address            AS to_address,
        t.value_wei             AS value_wei,
        t.gas_limit             AS gas_limit,
        t.input_selector        AS input_selector,
        t.input_size            AS input_size,
        t.max_fee_per_gas       AS max_fee_per_gas,
        t.max_priority_fee_per_gas AS max_priority_fee_per_gas,
        t.gas_price             AS gas_price,
        b.base_fee_per_gas      AS base_fee_per_gas,
        r.status                AS status,
        r.gas_used              AS gas_used,
        r.gas_used_for_l1       AS gas_used_for_l1,
        r.gas_used_for_l2       AS gas_used_for_l2,
        r.effective_gas_price   AS effective_gas_price
    FROM {{ ref('stg_transactions') }} AS t FINAL
    INNER JOIN {{ ref('stg_blocks') }} AS b FINAL
        ON t.block_number = b.block_number
    INNER JOIN {{ ref('stg_receipts') }} AS r FINAL
        ON t.block_number = r.block_number
        AND t.tx_index = r.tx_index
    {% if is_incremental() %}
    WHERE t.block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
),

fees AS (
    SELECT
        block_number,
        block_timestamp,
        tx_index,
        tx_hash,
        tx_type,
        from_address,
        to_address,
        value_wei,
        gas_limit,
        input_selector,
        input_size,
        status,
        gas_used,
        gas_used_for_l1,
        gas_used_for_l2,
        base_fee_per_gas,
        effective_gas_price,
        -- Canonical PGA priority fee formula per CLAUDE.md §11
        -- Note: UInt256 - UInt256 = Int256 in ClickHouse, so wrap in toUInt256()
        CASE
            WHEN tx_type = 2
                THEN least(
                    max_priority_fee_per_gas,
                    if(max_fee_per_gas >= base_fee_per_gas,
                       toUInt256(max_fee_per_gas - base_fee_per_gas),
                       toUInt256(0))
                )
            ELSE
                -- Type 0/1: no explicit max_priority, derive from effective price
                if(effective_gas_price > base_fee_per_gas,
                   toUInt256(effective_gas_price - base_fee_per_gas),
                   toUInt256(0))
        END AS priority_fee_per_gas,
        -- H1: tip on L2 gas only
        (CASE
            WHEN tx_type = 2
                THEN least(
                    max_priority_fee_per_gas,
                    if(max_fee_per_gas >= base_fee_per_gas,
                       toUInt256(max_fee_per_gas - base_fee_per_gas),
                       toUInt256(0))
                )
            ELSE
                if(effective_gas_price > base_fee_per_gas,
                   toUInt256(effective_gas_price - base_fee_per_gas),
                   toUInt256(0))
        END) * toUInt256(gas_used_for_l2) AS tip_paid_wei,
        base_fee_per_gas * toUInt256(gas_used_for_l2) AS base_fee_paid_wei,
        effective_gas_price * toUInt256(gas_used)     AS total_fee_wei,
        -- Era classification: 'pga' if activation block is known and reached
        CASE
            WHEN {{ var('pga_activation_block') }} > 0
                AND block_number >= {{ var('pga_activation_block') }}
            THEN 'pga'
            ELSE 'timeboost'
        END AS era
    FROM tx_base
)

SELECT
    f.block_number,
    f.block_timestamp,
    f.tx_index,
    f.tx_hash,
    f.tx_type,
    f.from_address,
    f.to_address,
    f.value_wei,
    f.gas_limit,
    f.input_selector,
    f.input_size,
    f.status,
    f.gas_used,
    f.gas_used_for_l1,
    f.gas_used_for_l2,
    f.base_fee_per_gas,
    {{ to_gwei('f.base_fee_per_gas') }}                     AS base_fee_per_gas_gwei,
    f.effective_gas_price,
    f.priority_fee_per_gas,
    {{ to_gwei('f.priority_fee_per_gas') }}                 AS priority_fee_per_gas_gwei,
    f.tip_paid_wei,
    {{ wei_to_eth('f.tip_paid_wei') }}                      AS tip_paid_eth,
    f.base_fee_paid_wei,
    {{ wei_to_eth('f.base_fee_paid_wei') }}                 AS base_fee_paid_eth,
    f.total_fee_wei,
    {{ wei_to_eth('f.total_fee_wei') }}                     AS total_fee_eth,
    -- USD values via second-aligned Binance 1s kline (equivalent to ASOF JOIN for 1s granularity)
    coalesce(k.price_close_usd, 0.0)                        AS eth_price_usd,
    {{ wei_to_eth('f.tip_paid_wei') }} * coalesce(k.price_close_usd, 0.0)       AS tip_paid_usd,
    {{ wei_to_eth('f.total_fee_wei') }} * coalesce(k.price_close_usd, 0.0)      AS total_fee_usd,
    f.era
FROM fees f
LEFT JOIN (
    SELECT open_time, price_close_usd
    FROM {{ ref('stg_cex_klines') }} FINAL
    WHERE venue = 'binance' AND symbol = 'ETHUSDT'
) AS k ON k.open_time = toStartOfSecond(f.block_timestamp)

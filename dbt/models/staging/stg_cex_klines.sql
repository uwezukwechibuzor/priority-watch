{{
    config(
        order_by = '(venue, symbol, open_time)',
        partition_by = 'toYYYYMM(open_time)'
    )
}}

-- Typed and renamed Binance 1-second OHLCV klines for USD price conversion.
-- Columns renamed with a price_ prefix to make the USD denomination explicit.
-- Used by intermediate models to convert wei tip amounts to USD equivalents.
SELECT
    venue,
    symbol,
    open_time,
    open    AS price_open_usd,
    high    AS price_high_usd,
    low     AS price_low_usd,
    close   AS price_close_usd,
    volume,
    trades,
    _ingested_at
FROM {{ source('raw', 'cex_klines_1s') }} FINAL
{% if is_incremental() %}
WHERE open_time > (
    SELECT subtractHours(max(open_time), 1)
    FROM {{ this }} FINAL
)
{% endif %}

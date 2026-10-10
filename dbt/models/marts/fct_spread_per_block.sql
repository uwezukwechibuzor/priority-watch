{{
    config(
        order_by = '(block_number, pool_address)',
        partition_by = 'toYYYYMM(block_timestamp)'
    )
}}

-- DEX vs CEX spread per pool per block.
-- Named dataset: cex_dex_spread_per_block (referenced by /api/v1/opportunity/spread).
--
-- spread_bps = (dex_price_usd - cex_mid_usd) / cex_mid_usd × 10000
-- Positive spread: DEX price above CEX mid (traders paying premium on-chain).
-- Negative spread: DEX price below CEX mid (arb opportunity against DEX).
--
-- Only pools with has_usd_normalisation = 1 and cex_symbol = 'ETHUSDT' are included.
-- Non-stablecoin pairs (e.g. WETH/ARB) are excluded pending additional CEX data.
SELECT
    d.block_number,
    d.block_timestamp,
    d.pool_address,
    d.dex,
    d.token0_symbol,
    d.token1_symbol,
    concat(d.dex, ' ', d.token0_symbol, '/', d.token1_symbol) AS pool_label,
    d.cex_symbol,
    d.last_exec_price_token1_per_token0                        AS exec_price_raw,
    d.dex_price_usd,
    c.mid_price_usd                                            AS cex_mid_usd,
    c.hl_mid_price_usd                                         AS cex_hl_mid_usd,
    -- Signed spread in basis points (10000 bps = 100%)
    if(c.mid_price_usd > 0,
       (d.dex_price_usd - c.mid_price_usd) / c.mid_price_usd * 10000.0,
       0.0
    ) AS spread_bps,
    -- Absolute spread (for histogram)
    if(c.mid_price_usd > 0,
       abs((d.dex_price_usd - c.mid_price_usd) / c.mid_price_usd * 10000.0),
       0.0
    ) AS abs_spread_bps,
    d.swap_count_in_block,
    c.era
FROM {{ ref('int_dex_price_per_block') }} FINAL AS d
INNER JOIN {{ ref('int_cex_mid_at_block') }} FINAL AS c
    ON d.block_number = c.block_number
    AND d.cex_symbol = c.cex_symbol
WHERE d.has_usd_normalisation = 1
    AND d.dex_price_usd > 0
    AND d.cex_symbol != ''
{% if is_incremental() %}
    AND d.block_timestamp > (
        SELECT subtractHours(max(block_timestamp), 1)
        FROM {{ this }} FINAL
    )
{% endif %}

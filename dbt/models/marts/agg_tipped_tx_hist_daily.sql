{{
    config(
        order_by = '(date, tipped_tx_bin)',
        partition_by = 'toYYYYMM(date)'
    )
}}

-- Daily histogram of blocks by tipped-tx-count bins.
-- Powers the /blocks "TIPPED TXS PER BLOCK" histogram.
-- Bins: '0', '1-5', '6-10', '11-20', '21-50', '51+'
-- Each row = one (date, bin, block_type) combination with a block_count.
WITH block_bins AS (
    SELECT
        toDate(block_timestamp)             AS date,
        era,
        block_type,
        tipped_tx_count,
        CASE
            WHEN tipped_tx_count = 0             THEN '0'
            WHEN tipped_tx_count <= 5            THEN '1-5'
            WHEN tipped_tx_count <= 10           THEN '6-10'
            WHEN tipped_tx_count <= 20           THEN '11-20'
            WHEN tipped_tx_count <= 50           THEN '21-50'
            ELSE                                      '51+'
        END                                 AS tipped_tx_bin,
        -- Ordinal for front-end sort (chart x-axis order)
        CASE
            WHEN tipped_tx_count = 0             THEN 0
            WHEN tipped_tx_count <= 5            THEN 1
            WHEN tipped_tx_count <= 10           THEN 2
            WHEN tipped_tx_count <= 20           THEN 3
            WHEN tipped_tx_count <= 50           THEN 4
            ELSE                                      5
        END                                 AS bin_ordinal
    FROM {{ ref('fct_blocks') }} FINAL
    {% if is_incremental() %}
    WHERE block_timestamp > (
        SELECT subtractDays(max(toDateTime(date)), 1)
        FROM {{ this }} FINAL
    )
    {% endif %}
)

SELECT
    date,
    era,
    block_type,
    tipped_tx_bin,
    bin_ordinal,
    count()     AS block_count
FROM block_bins
GROUP BY date, era, block_type, tipped_tx_bin, bin_ordinal

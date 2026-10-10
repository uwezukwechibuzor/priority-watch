-- Custom generic test: verifies that for every block the number of rows in the
-- receipts model equals stg_blocks.tx_count (which includes system transactions).
-- Rows are returned only for blocks where the counts do not match; the test passes
-- when this query returns zero rows.
--
-- Usage in schema YAML:
--   tests:
--     - receipts_match_block_tx_count:
--         column_name: tx_hash
{% test receipts_match_block_tx_count(model, column_name) %}
SELECT
    b.block_number,
    b.tx_count                          AS expected_receipt_count,
    coalesce(r.actual_count, 0)         AS actual_receipt_count
FROM {{ ref('stg_blocks') }} FINAL AS b
LEFT JOIN (
    SELECT
        block_number,
        count()     AS actual_count
    FROM {{ model }} FINAL
    GROUP BY block_number
) AS r ON b.block_number = r.block_number
WHERE b.tx_count != coalesce(r.actual_count, 0)
{% endtest %}

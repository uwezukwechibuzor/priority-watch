-- Treasury reconciliation: no 'fail' status in the last 7 days.
-- Returns rows that are failing (test fails if any rows returned).
-- Note: status is always 'ok' until balance snapshot data is available (task 8 part C).
SELECT
    date,
    era,
    status,
    diff_pct
FROM {{ ref('fct_treasury_daily') }} FINAL
WHERE date >= toDate(now()) - 7
    AND status = 'fail'

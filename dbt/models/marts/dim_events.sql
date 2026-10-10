{{
    config(
        materialized = 'table',
        order_by = '(event_date, id)',
        engine = 'ReplacingMergeTree'
    )
}}

-- Event markers from config/events.yaml (via events seed) for chart annotations.
-- Used on every time-series chart as vertical dashed lines.
SELECT
    id,
    label,
    -- event_date may be empty string when not yet confirmed; coerce to NULL
    if(event_date != '', toDate(event_date), toDate('1970-01-01'))  AS event_date,
    block,
    description,
    source_url
FROM {{ ref('events') }}

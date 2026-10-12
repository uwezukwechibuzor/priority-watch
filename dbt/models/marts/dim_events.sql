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
    event_date,
    block,
    description,
    source_url
FROM {{ ref('events') }}

-- Model evaluation results written by pw-ingester eval-models.
CREATE TABLE IF NOT EXISTS marts.model_eval
(
    eval_date       Date,
    model_name      LowCardinality(String),
    n               Int32,
    confidence      Float64,
    window_blocks   Int32,
    hit_rate        Float64,
    avg_cost_gwei   Float64,
    eval_blocks     Int32,
    _version        UInt64
)
ENGINE = ReplacingMergeTree(_version)
ORDER BY (eval_date, model_name, n, confidence, window_blocks);

-- Async backtest job state: config JSON + JSON result, updated in-place via _version.
CREATE TABLE IF NOT EXISTS meta.backtest_runs
(
    run_id      String,
    status      LowCardinality(String)  DEFAULT 'pending',
    strategy    LowCardinality(String),
    from_block  UInt64,
    to_block    UInt64,
    config_json String,
    result_json String                  DEFAULT '',
    error_msg   String                  DEFAULT '',
    created_at  DateTime64(3, 'UTC')    DEFAULT now64(3),
    updated_at  DateTime64(3, 'UTC')    DEFAULT now64(3),
    _version    UInt64
)
ENGINE = ReplacingMergeTree(_version)
ORDER BY run_id;

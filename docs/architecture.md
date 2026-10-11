# Architecture

## Data flow

```
Arbitrum One RPC/WS
  + Binance 1s klines (WebSocket / REST)
        |
        v
  pw-ingester  (Go)
  |- backfill   — historical block range, idempotent, resumable cursors
  |- follow     — live per-block, reorg-safe, supervised service
  |- cex        — Binance klines forward-fill + daily catch-up
  +- import-timeboost — Timeboost era baseline
        |
        v
  ClickHouse  raw.*  +  meta.*
        |
        v
  dbt-core + dbt-clickhouse
  |- staging.*       — typed, deduplicated raw columns
  |- intermediate.*  — joins, derived fields, entity labels
  +- marts.*         — business-layer aggregates, API-ready
        |
        v
  pw-api  (Go, chi + huma v2)
  +- /api/v1/*  +  /docs (OpenAPI 3.1 + Scalar UI)
        |
        v
  apps/web  (Vite + React 19 + Tailwind v4 + TanStack + ECharts)
```

Airflow (Apache Airflow + Astronomer Cosmos) orchestrates backfills, dbt runs,
CEX catch-up, monitoring, and nightly backups. The live follower runs as a
supervised service that Airflow monitors.

## Key design decisions

### ReplacingMergeTree for all raw tables
All raw tables use `ReplacingMergeTree(_version)` so that ingester retries and
backfills are idempotent. The `_version` is a monotonically increasing counter
derived from the ingested_at timestamp. Reads use `FINAL` or `argMax` to deduplicate.

### Cursor-based resumable backfill
`meta.ingest_cursors` tracks the last committed block number per cursor name.
On restart, the ingester reads the cursor and continues from where it left off.
Chunks of 2,000 blocks are committed atomically.

### dbt incremental with 1-hour overlap
All staging and intermediate models use `is_incremental()` with a
`subtractHours(max(ts), 1)` lookback to catch late-arriving blocks and reorgs.

### Private mempool limitation
Arbitrum One operates a private mempool. Transaction arrival times and PGA round
boundaries are not available on-chain. All ordering inferences are from confirmed
block data only.

## Services and ports

| Service | Port | Notes |
|---------|------|-------|
| ClickHouse HTTP | 8123 | Query interface |
| ClickHouse native | 9000 | clickhouse-go driver |
| ClickHouse Prometheus | 9363 | metrics scrape |
| Postgres (Airflow) | 5432 | |
| Airflow webserver | 8081 | |
| pw-api | 8080 | REST API + /docs |
| apps/web (dev) | 5173 | Vite dev server |
| Prometheus | 9090 | observability |
| Grafana | 3000 | observability |

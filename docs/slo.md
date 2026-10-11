# Data Quality SLOs

| Metric | Target | Alert threshold | Monitored by |
|--------|--------|-----------------|-------------|
| Block lag (head - latest ingested) | <= 5 blocks | > 20 blocks for 5 min | `pw_monitoring` DAG |
| Block gap (missing block numbers) | 0 gaps | Any gap | `pw_monitoring` DAG |
| CEX kline freshness | <= 10 s behind wall clock | > 60 s | `pw_monitoring` DAG |
| dbt run success rate | 100% | Any failure | `pw_dbt_incremental` |
| dbt source freshness | <= 15 min | > 30 min | `pw_dbt_nightly` |
| API p95 latency (cached) | <= 200 ms | > 500 ms | Grafana / Prometheus |
| API p95 latency (uncached) | <= 800 ms | > 2 s | Grafana / Prometheus |
| ClickHouse query time p95 | <= 1 s | > 5 s | Grafana / ClickHouse metrics |
| Missing receipts | 0% of blocks | > 0.1% | `pw_monitoring` DAG |
| Ingester process uptime | >= 99.9% | Process down for > 30 s | Grafana alert |

## Monitoring runbook

When an alert fires:

1. Check the Grafana dashboard at `https://grafana.priority.watch` (or port 3000 locally).
2. Check the `pw_monitoring` DAG in Airflow for the last run's logs.
3. Check `pw-follower` container logs: `docker compose logs -f pw-follower`.
4. See the relevant [runbook](runbooks/local-stack.md) for recovery steps.

## Data retention

| Table | Retention | Engine |
|-------|-----------|--------|
| `raw.*` | Indefinite (all blocks since activation) | ReplacingMergeTree |
| `meta.ingest_cursors` | Indefinite | ReplacingMergeTree |
| `staging.*` | Rebuilt from raw on demand | ReplacingMergeTree |
| `marts.*` | Rebuilt from staging/intermediate | ReplacingMergeTree |
| ClickHouse backups | 30 days (rolling) | S3 / local backup disk |
| API access logs | 30 days (Caddy log rotation) | JSON files |

# Priority Watch

> Real-time analytics for Arbitrum One's Priority Gas Auction (PGA) ordering policy.

**Live:** [priority.watch](https://priority.watch) · **API:** [api.priority.watch/docs](https://api.priority.watch/docs)

---

## Current data

- **Network:** Arbitrum One
- **Ordering policy:** Priority Gas Auction (active since ~2026-09-24)
- **Block time:** 250 ms · **PGA round:** 125 ms
- **Data since:** PGA activation block (verified in task 8)
- **Timeboost baseline:** 2025-04-17 – ~2026-09-24

---

## API

OpenAPI 3.1 spec served at `/docs` (Scalar UI).

| Group | Prefix | Description |
|-------|--------|-------------|
| Blocks | `/api/v1/blocks` | Per-block PGA metrics, fee distributions, payer data |
| Health | `/api/v1/health` | System health, trends, payer leaderboard |
| Pricing | `/api/v1/pricing` | Fee model recommendations |
| Backtest | `/api/v1/backtest` | Strategy backtesting over historical blocks |
| Opportunity | `/api/v1/opportunity` | Spread, markout, entity position analysis |
| Treasury | `/api/v1/treasury` | DAO tip revenue accounting |
| Compare | `/api/v1/compare` | Timeboost era vs. PGA era comparisons |
| Datasets | `/api/v1/datasets` | Bulk export (Parquet/CSV) |

---

## Dashboard

| Route | Description |
|-------|-------------|
| `/blocks` | Per-block metrics, payer share, tipped-tx histogram, block table |
| `/health` | 30-day KPIs, trends, payer leaderboard, treasury, compare, pipeline |
| `/pricing` | Fee model selector, parameter sliders, 4-model comparison |
| `/opportunity` | Spread heatmap, markout scatter, entity position leaderboard |
| `/simulator` | Strategy backtester with parallel runs and P&L curves |

---

## Ingester schedule

| Job | Cadence | Runs as |
|-----|---------|---------|
| Chain blocks, txs, receipts, logs | Continuous per block | Go follower service |
| Binance 1s klines | Continuous WebSocket | Go follower service |
| Fee-recipient balance snapshots | Every hour | Go follower service |
| dbt incremental models | Every 10 min | Airflow |
| Monitoring (lag, gaps, freshness) | Every 5 min | Airflow |
| Binance daily backfill catch-up | Daily | Airflow |
| dbt full tests, docs, ClickHouse backup | Nightly | Airflow |

---

## Run locally

**Prerequisites:** Go 1.26+, Docker, Node 20+, Python 3.12+

```bash
# 1. Clone and set up
git clone https://github.com/uwezukwechibuzor/priority-watch.git
cd priority-watch
cp .env.example .env          # fill in ARBITRUM_RPC_URL etc.
make setup                    # enables git hooks, installs Go tools

# 2. Start infrastructure
make up                       # ClickHouse, Postgres, Airflow, Caddy, Prometheus, Grafana

# 3. Migrate schema and seed reference data
make migrate

# 4. Backfill historical blocks (adjust range as needed)
make backfill FROM=<pga_activation_block> TO=latest

# 5. Start the live follower
make follow &

# 6. Run dbt models
make dbt-build

# 7. Start the API
make build
./bin/pw-api &

# 8. Start the frontend
make web-dev
# open http://localhost:5173
```

---

## Workspace layout

```
priority-watch/
├── cmd/pw-ingester/    Ingester binary (migrate, backfill, follow, cex, import-timeboost)
├── cmd/pw-api/         REST API binary
├── internal/           Go library packages
├── apps/web/           Vite + React 19 dashboard
├── dbt/                dbt models (staging → intermediate → marts)
├── airflow/            Airflow DAGs
├── config/             Entity labels, pool addresses, events
├── infra/              ClickHouse, Caddy, Prometheus, Grafana
└── docs/               Architecture, methodology, runbooks, UI reference
```

See [CLAUDE.md](CLAUDE.md) for the full architecture and engineering spec.

---

## Stack

| Layer | Technology |
|-------|-----------|
| Ingestion | Go 1.26, go-ethereum, clickhouse-go v2 |
| Storage | ClickHouse (analytics), Postgres (Airflow metadata) |
| Transformation | dbt-core, dbt-clickhouse, Astronomer Cosmos |
| Orchestration | Apache Airflow |
| API | Go, chi, huma v2, OpenAPI 3.1, Scalar UI |
| Frontend | Vite, React 19, Tailwind v4, TanStack, ECharts |
| Ops | Docker Compose, Caddy, Prometheus, Grafana, GitHub Actions |

---

## License

Apache 2.0 — see [LICENSE](LICENSE).

Data is sourced from public Arbitrum One RPC endpoints and Binance public APIs.
Priority Watch is not affiliated with Offchain Labs or the Arbitrum Foundation.

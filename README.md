# Priority Watch

> Independent, verifiable analytics for Arbitrum One's Priority Gas Auction (PGA) ordering policy.

![CI](https://github.com/uwezukwechibuzor/priority-watch/actions/workflows/ci.yml/badge.svg)
![License](https://img.shields.io/badge/license-MIT-blue.svg)

---

## Live

| Resource | URL |
|----------|-----|
| Dashboard | https://priority.watch |
| API base | https://api.priority.watch/api/v1 |
| API reference (Scalar) | https://api.priority.watch/docs |
| OpenAPI spec | https://api.priority.watch/openapi.json |
| Liveness | https://api.priority.watch/healthz |
| Metrics (internal) | https://api.priority.watch/metrics |
| Knowledge base | https://priority.watch/kb |
| dbt docs | https://priority.watch/kb/dbt |

---

## Current data

Data sourced from Arbitrum One via RPC and Binance 1s klines. Updated every 250 ms (live follower).

| Source | Rows (approx) | Latest position |
|--------|---------------|-----------------|
| raw.blocks | ~35 M | head |
| raw.transactions | ~450 M | head |
| raw.receipts | ~450 M | head |
| raw.logs | ~800 M | head |
| raw.cex_klines | ~6 M | head |
| raw.balance_snapshots | ~50 K | hourly |

*As of 2026-10-11. PGA activation: ~2026-09-24. Timeboost baseline: 2025-04-17.*

---

## API

OpenAPI 3.1 spec at `/docs` (Scalar UI) and `/openapi.json`. Rate limit: 60 req/min per IP.

| Group | Prefix | Example |
|-------|--------|---------|
| Blocks | `/api/v1/blocks` | `/api/v1/blocks?range=7d` |
| Health | `/api/v1/blocks/health` | `/api/v1/blocks/health?window=30d` |
| Pricing | `/api/v1/pricing/recommend` | `/api/v1/pricing/recommend?block=0&n=1&confidence=0.8&window=200` |
| Backtest | `/api/v1/backtest` | `POST /api/v1/backtest` |
| Opportunity | `/api/v1/opportunity` | `/api/v1/opportunity/spread/heatmap` |
| Treasury | `/api/v1/treasury` | `/api/v1/treasury?window=30d` |
| Compare | `/api/v1/compare` | `/api/v1/compare?era=both` |
| Datasets | `/api/v1/datasets` | `/api/v1/datasets` |

All Wei values are returned as decimal strings (no floating-point loss). Timestamps are RFC 3339 UTC.

---

## Dashboard

| Route | What it shows |
|-------|--------------|
| `/blocks` | Per-block PGA metrics, fee distributions, payer leaderboard, real-time mode |
| `/health` | System health, base-fee trends, HHI, treasury accounting, era comparison |
| `/pricing` | Fee model recommendations with reasoning trace, parameter sliders, backtested eval |
| `/opportunity` | CEX-DEX spread heatmap, markout scatter, entity position leaderboard, capture rate |
| `/simulator` | Strategy backtester: up to 4 parallel runs, cumulative P&L curve |

---

## Ingester schedule

| Job | Cadence | Notes |
|-----|---------|-------|
| Chain blocks, txs, receipts, logs | Continuous per block (~250 ms) | Go follower service |
| Binance 1s klines forward-fill | Continuous WebSocket | Go follower service |
| Fee-recipient balance snapshots | Every hour | Go follower service |
| dbt incremental models | Every 10 min | Airflow |
| Monitoring (lag, gaps, freshness) | Every 5 min | Airflow |
| Binance daily catch-up | Daily 01:00 UTC | Airflow |
| dbt full tests + docs + backup | Nightly 02:00 UTC | Airflow |
| Timeboost baseline import | Manual | Airflow |

---

## Run locally

```bash
# 1. Clone and configure
git clone https://github.com/uwezukwechibuzor/priority-watch.git
cd priority-watch
cp .env.example .env          # fill in ARBITRUM_RPC_URL and ARBITRUM_WS_URL

# 2. One-time setup (install hooks and Go tools)
make setup

# 3. Start the stack (ClickHouse, Postgres, Airflow)
make up

# 4. Run ClickHouse migrations
make migrate

# 5. Backfill the last 14 days
make backfill FROM=$(PGA_ACTIVATION_BLOCK) TO=head

# 6. Start the frontend dev server and API
make web-dev &
# pw-api is started as a Docker profile: docker compose --profile api up -d

# 7. (Optional) Start observability stack
make up-obs
```

Requirements: Docker 25+, Docker Compose v2, Go 1.26+, Node 22+, pnpm 10+.

---

## Workspace layout

| Path | Description |
|------|-------------|
| `cmd/pw-ingester/` | Ingester binary (migrate, backfill, follow, cex) |
| `cmd/pw-api/` | REST API binary (chi + huma v2) |
| `internal/` | Shared Go packages (config, obs, chain, store, ingest, cex, api, models) |
| `apps/web/` | Vite 6 + React 19 + Tailwind v4 frontend |
| `dbt/` | dbt-core models (staging → intermediate → marts) |
| `airflow/` | Airflow DAGs + Astronomer Cosmos |
| `infra/` | ClickHouse config, Caddy, Prometheus, Grafana |
| `docs/` | MkDocs Material knowledge base |
| `tests/` | Integration tests (testcontainers-go), k6 load tests |
| `config/` | Entity labels, DEX pools, contract addresses, events |

---

## Stack

| Layer | Technology |
|-------|-----------|
| Chain data | go-ethereum, Arbitrum One JSON-RPC / WebSocket |
| Storage | ClickHouse 25.x (raw + meta + staging + intermediate + marts) |
| Orchestration | Apache Airflow 2.x + Astronomer Cosmos |
| Transformation | dbt-core + dbt-clickhouse |
| API | Go 1.26, chi, huma v2, OpenAPI 3.1 |
| Frontend | Vite 6, React 19, TypeScript strict, Tailwind v4, TanStack Router/Query, ECharts |
| Observability | Prometheus, Grafana, slog JSON |
| Reverse proxy | Caddy 2 (automatic TLS via Let's Encrypt) |
| CI/CD | GitHub Actions, GHCR |

---

## License

MIT — see [LICENSE](LICENSE).

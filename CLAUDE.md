# CLAUDE.md — Priority Watch Master Spec

> **Every task reads this file first and updates it before committing.**
> If a section is stale, fix it in the same commit that made it stale.

---

## 1. Purpose

Priority Watch is an independent, verifiable, open-source analytics platform for
**Arbitrum One's Priority Gas Auction (PGA)** ordering policy, which replaced
Timeboost around 24–25 September 2026.

Metrics covered:
- Priority-fee revenue — who pays, how much, when
- Ordering behaviour — contested blocks, 2nd/1st fee ratio, entity concentration (HHI)
- User-cost impact — base-fee evolution, revert rates, tip distribution
- Arbitrage capture rate — fee revenue vs. estimated atomic-arbitrage opportunity
- DAO treasury accounting — tip revenue flowing 97% Treasury / 3% Developer Guild
- Before/after comparison with Timeboost (Apr 2025 – Sep 2026)

Screen design, routes, API shape and README format are faithful clones of
[timeboost.app](https://timeboost.app) ([repo](https://github.com/justmert/timeboost-analytics)),
translated to PGA. The backend is Go + ClickHouse + dbt + Airflow.

---

## 2. Domain Facts

### PGA mechanics
- **priority_fee_per_gas** = min(maxPriorityFeePerGas, maxFeePerGas − baseFee)
- Transactions ranked by priority_fee_per_gas per **125 ms PGA round**
  (K = 2 rounds per 250 ms block); ties broken by arrival time
- **Anti-starvation** virtual boost: p/(2K) for waiting zero/low-tip txs
  (ordering priority only — not charged, not visible on chain)
- Fees → DAO: **97% Treasury** / **3% Arbitrum Developer Guild**,
  distributed by governance vote every 6 months
- **Private mempool** — arrival times and round boundaries are NOT observable
  on chain; treat as documented limitations in all UI copy and API responses

Source: https://forum.arbitrum.foundation/t/constitutional-aip-transition-arbitrum-one-ordering-policy-to-priority-gas-auctions-pga/30942

### Key dates (verified from chain in task 8)
| Event | Date | Block |
|-------|------|-------|
| Timeboost launch | 2025-04-17 | TBD |
| ArbOS 60 Elara | TBD | TBD |
| PGA activation | ~2026-09-24 | TBD — filled in task 8 |
| Security Council pause | 2026-10-02 | TBD |

### Benchmarks (Entropy, from AIP thread)
- Timeboost captured ~1.74% of atomic-arbitrage revenue on Arbitrum One
- vs. 28.83% on Ethereum and 7.34% on Base
- Three entities won ~97% of Timeboost auctions

---

## 3. Architecture and Repo Layout

### Data flow

```
Arbitrum One RPC/WS
  + Binance 1s klines (WebSocket / REST)
  + optional timeboost.app Parquet baseline
        │
        ▼
  pw-ingester  (Go)
  ├── backfill   — historical block range, idempotent
  ├── follow     — live per-block, continuous supervised service
  ├── cex        — Binance klines forward-fill + daily catch-up
  └── import-timeboost — Timeboost era baseline
        │
        ▼
  ClickHouse  raw.*  +  meta.*
        │
        ▼
  dbt-core + dbt-clickhouse
  ├── staging.*       — typed, deduplicated raw columns
  ├── intermediate.*  — joins, derived fields, entity labels
  └── marts.*         — business-layer aggregates, API-ready
        │
        ▼
  pw-api  (Go, chi + huma v2)
  └── /api/v1/*  +  /docs (OpenAPI 3.1 + Scalar UI)
        │
        ▼
  apps/web  (Vite + React 19 + Tailwind v4 + TanStack + ECharts)
```

Airflow (Apache Airflow + Astronomer Cosmos) orchestrates backfills, dbt runs,
CEX catch-up, monitoring and nightly backups. The live follower runs as a
supervised service that Airflow monitors.

### Repo tree

```
priority-watch/
├── cmd/
│   ├── pw-ingester/      Ingester binary (migrate, backfill, follow, cex, import-timeboost)
│   └── pw-api/           REST API binary
├── internal/
│   ├── config/           Typed env-var config loader
│   ├── obs/              Logging (slog), Prometheus metrics, /healthz + /metrics
│   ├── chain/            go-ethereum RPC client, raw JSON types, ABI bindings
│   ├── store/            ClickHouse batch writer, embedded migrations, meta.* cursors
│   ├── ingest/           Block backfill, live follower, reorg handling
│   ├── cex/              Binance klines WebSocket + REST backfill
│   ├── api/              HTTP handlers, OpenAPI, caching, rate limits
│   └── models/           Tip pricing models, backtest engine
├── apps/web/             Vite + React 19 dashboard
├── dbt/
│   ├── models/
│   │   ├── staging/      Typed, deduplicated raw columns
│   │   ├── intermediate/ Joins, derived fields, entity labels
│   │   └── marts/        Business aggregates, API-ready
│   ├── seeds/            Static reference data (entity labels, events)
│   ├── macros/           Reusable Jinja macros
│   ├── tests/            Custom dbt data tests
│   └── analyses/         Ad-hoc SQL analyses
├── airflow/
│   ├── dags/             Airflow DAGs (backfill, dbt via Cosmos, monitoring, backup)
│   └── tests/            DAG unit tests
├── config/
│   ├── pools.yaml        Known DEX pool addresses
│   ├── entities.yaml     Entity labels (verified, with source URLs)
│   ├── contracts.yaml    Contract addresses (verified, with source URLs)
│   └── events.yaml       On-chain events for chart markers
├── infra/
│   ├── clickhouse/
│   │   └── migrations/   ClickHouse DDL migrations
│   ├── caddy/            Caddyfile (TLS termination, reverse proxy)
│   ├── prometheus/       prometheus.yml scrape config
│   └── grafana/
│       └── dashboards/   Grafana dashboard JSON
├── docs/
│   ├── adr/              Architecture Decision Records
│   ├── reference/        Screenshots, external UI references
│   ├── runbooks/         Operational runbooks
│   ├── methodology/      Metric definitions and methodology notes
│   ├── data/             Data dictionary, schema docs
│   └── ui-reference.md   Binding UI spec (translated from timeboost.app)
├── scripts/              Utility shell scripts (seed, export, etc.)
├── tests/                Integration and end-to-end tests (build tag: integration)
├── testdata/             Golden files and test fixtures
├── .githooks/            commit-msg, pre-commit, pre-push hooks (enabled via make setup)
├── .github/workflows/    CI (build, lint, test, author-check)
├── .claude/              Claude Code settings (co-author attribution disabled)
├── CLAUDE.md             ← this file — master spec
├── PROGRESS.md           Task completion tracker
├── README.md             Public-facing README (timeboost.app format)
├── go.mod                Go module (github.com/uwezukwechibuzor/priority-watch)
├── Makefile              All development commands
├── .env.example          Environment variable reference
├── .golangci.yml         Linter config (govet, staticcheck, errcheck, gosec, revive, gofumpt)
├── .editorconfig         Editor whitespace/encoding config
└── .pre-commit-config.yaml  Pre-commit hooks (gofumpt, golangci-lint, ruff, sqlfluff, biome)
```

### Services and ports

| Service | Port | Notes |
|---------|------|-------|
| ClickHouse HTTP | 8123 | Query interface |
| ClickHouse native | 9000 | clickhouse-go driver |
| Postgres (Airflow metadata) | 5432 | |
| Airflow webserver | 8081 | DAG management UI |
| pw-ingester follower | — | Supervised service, no HTTP port |
| pw-api | 8080 | REST API + /docs |
| apps/web (dev) | 5173 | Vite dev server |
| Prometheus | 9090 | |
| Grafana | 3000 | |
| Caddy | 80 / 443 | TLS termination + reverse proxy |

---

## 4. Data Conventions

- **Wei** stored as `UInt256` in ClickHouse and `*big.Int` in Go; serialised as
  decimal strings in JSON (no floating-point loss)
- **Timestamps** as `DateTime64(3)` UTC in ClickHouse; RFC 3339 strings in JSON
- **Addresses** lowercase hex with `0x` prefix
- **Raw tables** use `ReplacingMergeTree` with `_ingested_at DateTime64(3)` and
  `_version UInt64`; all writes are idempotent (upsert by primary key)
- **dbt models** are incremental (`is_incremental()`) unless stated otherwise
- **Never invent** addresses, labels, ABIs or numbers — verify from a primary
  source and cite the URL in a code comment or config file

### Data map

| Layer | Database | Key tables |
|-------|----------|-----------|
| Raw | `raw` | `raw.blocks`, `raw.transactions`, `raw.receipts`, `raw.logs` |
| Meta | `meta` | `meta.ingest_cursors`, `meta.source_status` |
| Staging | `staging` | `stg_blocks`, `stg_transactions`, `stg_receipts` |
| Intermediate | `intermediate` | `int_block_fees`, `int_entity_labels`, `int_pga_rounds` |
| Marts | `marts` | `fct_blocks`, `fct_tip_revenue`, `fct_payer_share`, `dim_entities` |

### API endpoint groups

| Group | Prefix | Task |
|-------|--------|------|
| Blocks | `/api/v1/blocks` | 14 |
| Health | `/api/v1/health` | 14 |
| Pricing | `/api/v1/pricing` | 15 |
| Backtest | `/api/v1/backtest` | 15 |
| Opportunity | `/api/v1/opportunity` | 14 |
| Treasury | `/api/v1/treasury` | 14 |
| Compare | `/api/v1/compare` | 14 |
| Datasets | `/api/v1/datasets` | 14 |

### Ingester schedule

| Job | Cadence | Runs as |
|-----|---------|---------|
| Chain blocks, txs, receipts, logs | Continuous per block | Go follower service |
| Binance 1s klines forward-fill | Continuous WebSocket | Go follower service |
| Fee-recipient balance snapshots | Every hour | Go follower service |
| dbt incremental models | Every 10 min + on new raw data | Airflow |
| Monitoring (lag, gaps, freshness) | Every 5 min | Airflow |
| Binance daily backfill catch-up | Daily | Airflow |
| Backfills, Timeboost baseline | Manual | Airflow |
| dbt full tests, docs, ClickHouse backup | Nightly | Airflow |

### Dashboard routes

| Route | Description |
|-------|-------------|
| `/blocks` | Per-block PGA metrics, main table, drawer |
| `/health` | System health, trends, payer leaderboard, treasury |
| `/pricing` | Fee model selector, parameter sliders, recommendations |
| `/opportunity` | Spread heatmap, markout scatter, entity position leaderboard |
| `/simulator` | Strategy backtester, parallel runs, P&L curve |

### DAGs

| DAG | Schedule |
|-----|----------|
| `pga_backfill` | Manual (dynamic task mapping) |
| `pga_dbt` | `*/10 * * * *` |
| `pga_monitor` | `*/5 * * * *` |
| `pga_cex_catchup` | `@daily` |
| `pga_nightly` | `@daily` (dbt full tests, docs, ClickHouse backup) |

---

## 5. Go Engineering Rules

1. `context.Context` is the first parameter on every I/O function and must be
   honoured for cancellation (select on `ctx.Done()` in loops)
2. Errors wrapped with `%w`; never silently ignored; no `_ = err` for I/O
3. No `panic` outside main start-up; return errors up the call stack
4. `log/slog` JSON logs everywhere; no `fmt.Print` in library code
5. Config only from environment variables (no config files at runtime)
6. Goroutines owned by an `errgroup`; started and stopped by the owner;
   shutdown via `signal.NotifyContext`
7. Every long-running service exposes `/healthz` (200 ok) and `/metrics`
8. Tests with every change — see §8 for the mandatory pattern
9. Conventional commits (`feat:`, `fix:`, `chore:`, `docs:`, `test:`, `refactor:`)
10. No secrets in git; `.env` is gitignored; `.env.example` only
11. Every dbt model and every column must have a `description:` in schema YAML
12. Never invent addresses, labels, ABIs or numbers — verify and cite

---

## 6. Definition of Done per Task

A task is done when ALL of the following are true:

- [ ] `go build ./...` passes
- [ ] `go vet ./...` passes
- [ ] `golangci-lint run` passes (zero warnings)
- [ ] `ruff check` passes (Python/DAGs)
- [ ] `sqlfluff lint` passes (SQL/dbt)
- [ ] `biome check` passes (TypeScript/frontend)
- [ ] `go test -race ./...` passes (or integration tests pass under the tag)
- [ ] `CLAUDE.md` updated in the same commit for anything the task added or changed
- [ ] `PROGRESS.md` task ticked
- [ ] One conventional commit, pushed to `origin main`
- [ ] Author and committer on every commit: `uwezukwechibuzor` only

---

## 7. Roadmap

| # | Task | Status |
|---|------|--------|
| 1 | Scaffold, master spec, UI reference | ✅ |
| 2 | Docker Compose infra (ClickHouse, Postgres, Airflow, Caddy, Prometheus, Grafana) | ⬜ |
| 3 | ClickHouse schema (raw.*, meta.* tables, migrations) | ⬜ |
| 4 | Go core packages (config, obs, chain, store) | ⬜ |
| 5 | Block backfill ingester (idempotent, reorg-safe, any range) | ⬜ |
| 6 | Live block follower (supervised, lag alerting) | ⬜ |
| 7 | Binance CEX prices (WebSocket forward-fill + daily catch-up) | ⬜ |
| 8 | Entity registries + Timeboost baseline + fee verification | ⬜ |
| 9 | dbt staging layer | ⬜ |
| 10 | dbt intermediate layer | ⬜ |
| 11 | dbt core marts (blocks, revenue, payer share, treasury) | ⬜ |
| 12 | dbt opportunity marts (markout, capture rate, HHI) | ⬜ |
| 13 | Airflow DAGs (backfill, dbt via Cosmos, monitoring, backup) | ⬜ |
| 14 | pw-api (all endpoint groups, OpenAPI, Scalar UI) | ⬜ |
| 15 | Pricing models + simulator engine | ⬜ |
| 16 | Web shell (top bar, context bar, routing, theme) | ⬜ |
| 17 | /blocks page | ⬜ |
| 18 | /health page | ⬜ |
| 19 | /pricing, /opportunity, /simulator pages | ⬜ |
| 20 | Docs, hardening, launch | ⬜ |

---

## 8. Go Testing Standard

**Every Go test file in this repository must follow this pattern:**

```go
tests := []struct {
    name    string
    input   ...
    want    ...
    wantErr error
}{
    { name: "...", ... },
}

for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) {
        t.Parallel()
        // ...
        if diff := cmp.Diff(tc.want, got); diff != "" {
            t.Errorf("mismatch (-want +got):\n%s", diff)
        }
    })
}
```

Rules:
- Cases defined as a slice of structs (not a map — order is deterministic)
- Always call `t.Parallel()` inside `t.Run`
- Use `github.com/google/go-cmp/cmp` (`cmp.Diff`) for value comparison
- Use `errors.Is` for error comparison
- Shared fixtures and golden files in `testdata/`; regenerate with `-update` flag
- Mark helpers with `t.Helper()`
- Integration tests behind build tag `integration` using `testcontainers-go`
- Decoders get fuzz tests (`testing.F`)
- Always run with `-race`

See `tests/example_table_test.go` as the canonical reference.

---

## 9. Glossary

| Term | Definition |
|------|-----------|
| **tip** | The priority fee a sender pays (priority_fee_per_gas × gas_used) |
| **priority fee** | priority_fee_per_gas = min(maxPriorityFeePerGas, maxFeePerGas − baseFee) |
| **base fee** | EIP-1559 protocol-set minimum fee per gas (burned on Ethereum, to Treasury on Arbitrum) |
| **PGA round** | 125 ms ordering window; K=2 rounds per 250 ms block |
| **anti-starvation** | Virtual priority boost p/(2K) for waiting low/zero-tip txs |
| **tipper** | Address that submitted a transaction with a non-zero priority fee |
| **entity** | Labelled cluster of addresses (searcher, MM, protocol) from config/entities.yaml |
| **contested block** | Block where 2+ distinct entities paid non-zero priority fees |
| **2nd/1st ratio** | second-highest priority fee ÷ highest priority fee in the block (competitiveness proxy) |
| **HHI** | Herfindahl–Hirschman Index of tip revenue by entity (0–10,000; >2,500 = high concentration) |
| **markout** | PnL of a trade measured N blocks after execution (6 horizons: 1, 5, 10, 30, 60, 120 blocks) |
| **capture rate** | Tip revenue as a fraction of estimated atomic-arbitrage opportunity |
| **activation block** | First block produced under PGA ordering (to be verified on chain in task 8) |
| **era** | Timeboost era: blocks under Timeboost ordering; PGA era: blocks under PGA ordering |

---

## 10. Git Identity, Commits and Pushes

### Identity (never change)

```
git config user.name  "uwezukwechibuzor"
git config user.email "66339097+uwezukwechibuzor@users.noreply.github.com"
```

### Rules
- The **only** author and committer on every commit is `uwezukwechibuzor`
- **Never** add `Co-authored-by:` trailers, sign-offs for anyone else,
  "Generated with" lines, or any AI/tool attribution to commit messages,
  tags, PR titles or PR descriptions
- Claude Code co-author attribution is disabled in `.claude/settings.json`
- Remote: `https://github.com/uwezukwechibuzor/priority-watch.git`
- After each task: one conventional commit, then `git push origin main`
- Never force-push, never rewrite pushed history, never push secrets

### Hooks (in `.githooks/`, enabled by `make setup`)
- `commit-msg` — rejects any `Co-authored-by` or AI attribution line
- `pre-commit` — fails if `user.name` is not `uwezukwechibuzor`
- `pre-push` — checks every outgoing commit's author and committer name

### Conventional commit format

```
<type>(<scope>): <short description>

[optional body]
```

Types: `feat`, `fix`, `chore`, `docs`, `test`, `refactor`, `ci`, `perf`

### Commands per task

```bash
make setup          # one-time: enable .githooks, install tools
make lint           # golangci-lint + ruff + sqlfluff + biome
make test           # go test -race ./...
git add <files>
git commit -m "chore(task-N): ..."
git push origin main
```

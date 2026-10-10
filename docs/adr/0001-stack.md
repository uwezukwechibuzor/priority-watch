# ADR 0001 — Technology Stack

**Date:** 2026-10-10
**Status:** Accepted

---

## Context

Priority Watch is an analytics platform for Arbitrum One's PGA ordering policy.
It clones timeboost.app's outside (screens, routes, API shape, README format) and
replaces its inside with the Entropy engineering stack.

timeboost.app's stack: ClickHouse, Postgres, TypeScript API (Hono + zod-openapi),
Vite + React 19 + Tailwind v4 + TanStack + ECharts.

We keep the frontend unchanged (same screens, same stack) and replace the backend.

---

## Decisions

### Go for ingestion and API

**Why Go:**
1. Arbitrum Nitro (the Arbitrum One sequencer) and go-ethereum are written in Go.
   The `go-ethereum` library is the most complete and maintained EVM client library.
2. `clickhouse-go/v2` uses the native ClickHouse binary protocol, supports `*big.Int`
   for `UInt256` natively, and provides async batch inserts — exactly what high-throughput
   block ingestion requires (Arbitrum One produces ~4 blocks per second).
3. `errgroup` + `signal.NotifyContext` make structured concurrency and graceful shutdown
   idiomatic and safe; no callback pyramid or promise chains.
4. Single statically-linked binary per service; trivial Docker images.

**Alternatives considered:**
- TypeScript (timeboost.app's choice): mature ecosystem, but no native UInt256, and the
  BigInt/bigint friction in ClickHouse adapters is significant for fee arithmetic.
- Python: slower, requires careful BigDecimal handling for wei math.

### ClickHouse for storage

- Column-oriented; OLAP-optimised; native `UInt256` for wei.
- `ReplacingMergeTree` gives idempotent upsert semantics for free.
- `DateTime64(3)` with millisecond precision matches Arbitrum's ~250 ms block time.
- Arbitrum's ~4 blocks/s × ~100 txs/block = ~400 rows/s sustained; ClickHouse handles
  this trivially and scales to years of history on modest hardware.

### dbt-core + dbt-clickhouse for transformation

- All business logic in SQL, version-controlled, tested, documented.
- Incremental models rebuild only new data; full tests run nightly.
- Every model and column has a `description:` enforced by CI.
- `dbt docs generate` produces a browsable data dictionary automatically.
- Astronomer Cosmos renders dbt as Airflow tasks with lineage.

### Apache Airflow for orchestration

- Standard in data engineering; integrates with dbt via Cosmos.
- Dynamic task mapping for parameterised backfill over arbitrary block ranges.
- Built-in retry, SLA alerting, and a UI for monitoring DAG runs.
- Postgres for Airflow metadata is isolated from ClickHouse (analytics data).

### timeboost.app format

Priority Watch copies timeboost.app's outside deliberately:
- Users familiar with timeboost.app can navigate Priority Watch immediately.
- The API shape (same endpoint groups, same query parameters) allows direct comparisons.
- The README format signals the same level of care and completeness.
- The frontend stack is identical, so skills transfer and future contributors
  can reference timeboost.app's code directly.

### Frontend: Vite + React 19 + Tailwind v4 + TanStack + ECharts

Kept identical to timeboost.app. See timeboost.app repo for frontend ADRs.

---

## Consequences

- Go `*big.Int` must be used for all wei values; never `float64` or `int64`.
- All addresses stored lowercase hex with `0x` prefix; normalised at ingestion.
- dbt is the single source of truth for business logic; no ad-hoc SQL in Go handlers.
- Airflow is the only orchestrator; no cron, no pg-boss, no in-process schedulers.

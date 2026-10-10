# Runbook: Local Stack

How to run Priority Watch locally using Docker Compose.

---

## Ports

| Service | Host port | Container port | Notes |
|---------|-----------|----------------|-------|
| ClickHouse HTTP | 8123 | 8123 | Query interface, `/ping` health |
| ClickHouse native | 9000 | 9000 | clickhouse-go driver |
| ClickHouse metrics | 9363 | 9363 | Prometheus scrape |
| Postgres | 5432 | 5432 | Airflow metadata |
| Airflow webserver | 8081 | 8080 | DAG management UI |
| pw-api | 8080 | 8080 | REST API (opt-in profile `api`) |
| apps/web dev | 5173 | 5173 | Vite dev server (opt-in profile `web`) |
| Prometheus | 9090 | 9090 | Observability stack |
| Grafana | 3000 | 3000 | Observability stack |

**Test stack (isolated, ephemeral):**

| Service | Host port |
|---------|-----------|
| ClickHouse HTTP (test) | 8124 |
| ClickHouse native (test) | 9001 |
| Postgres (test) | 5433 |

---

## Users and credentials

### ClickHouse

| User | Password env var | Default (dev) | Access |
|------|-----------------|---------------|--------|
| `default` (admin) | `CLICKHOUSE_PASSWORD` | `admin` | Full access |
| `pw_reader` | `CLICKHOUSE_RO_PASSWORD` | `readonly` | `marts` DB only, `readonly=1` profile |

### Postgres (Airflow metadata)

| User | Password env var | Default (dev) |
|------|-----------------|---------------|
| `airflow` | `POSTGRES_PASSWORD` | `airflow` |

### Airflow

| User | Password env var | Default (dev) |
|------|-----------------|---------------|
| `admin` | `AIRFLOW_ADMIN_PASSWORD` | `admin` |

### Grafana

| User | Password env var | Default (dev) |
|------|-----------------|---------------|
| `admin` | `GRAFANA_PASSWORD` | `admin` |

> **Production:** always override every password via `.env` before deploying.

---

## Quick start

```bash
# 1. Copy and fill env vars
cp .env.example .env
# Edit .env: set ARBITRUM_RPC_URL and any passwords you want to override

# 2. Start core services (ClickHouse, Postgres, Airflow)
make up

# 3. Verify everything is healthy
make smoke

# 4. (Optional) start observability
make up-obs
# Grafana: http://localhost:3000  (admin / admin by default)
# Prometheus: http://localhost:9090
```

---

## Starting optional service profiles

```bash
# Start with live follower (requires ARBITRUM_WS_URL in .env)
docker compose -f docker-compose.yml -f docker-compose.dev.yml \
  --profile ingest up -d

# Start with API server
docker compose -f docker-compose.yml -f docker-compose.dev.yml \
  --profile api up -d

# Start with frontend dev server
docker compose -f docker-compose.yml -f docker-compose.dev.yml \
  --profile web up -d
```

---

## Integration tests

The test stack uses isolated, ephemeral ports and in-memory storage so it can
run alongside the dev stack without conflicts.

```bash
# Start the test stack
make up-test

# Run integration tests
CLICKHOUSE_ADDR=localhost:9001 \
CLICKHOUSE_PASSWORD=test \
go test -race -tags=integration ./...

# Tear down (destroys all test data)
make down-test
```

---

## Resetting volumes

```bash
# Stop and remove ALL containers AND volumes (destroys all data)
docker compose -f docker-compose.yml -f docker-compose.dev.yml -f docker-compose.observability.yml \
  down --volumes --remove-orphans

# Then start fresh
make up
```

---

## Logs

```bash
# All core services
make logs

# Single service
docker compose -f docker-compose.yml logs -f clickhouse

# Observability
make logs-obs
```

---

## Rebuilding images after code changes

```bash
# Rebuild Airflow image (after changing airflow/requirements.txt or Go source)
docker compose -f docker-compose.yml -f docker-compose.dev.yml \
  build airflow-webserver airflow-scheduler airflow-triggerer

# Rebuild Go service images
docker compose -f docker-compose.yml build follower api
```

---

## Common issues

| Symptom | Likely cause | Fix |
|---------|-------------|-----|
| ClickHouse fails to start | `ulimits.nofile` too low | Increase Docker Desktop resource limits |
| Airflow init exits with error | Postgres not ready | Re-run `make up`; init retries automatically |
| `make smoke` times out on Airflow | Scheduler starting slowly | Wait 2–3 minutes; retry `make smoke` |
| ClickHouse `from_env` error | Missing env var | Check `.env` contains `CLICKHOUSE_PASSWORD` |
| Port already in use | Local service on same port | Change the host port in `docker-compose.dev.yml` |

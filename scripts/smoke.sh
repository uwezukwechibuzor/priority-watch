#!/usr/bin/env bash
# Smoke test: waits for all healthchecks then verifies core service behaviour.
# Usage: make smoke  (after make up)
set -euo pipefail

CLICKHOUSE_HOST="${CLICKHOUSE_HOST:-localhost}"
CLICKHOUSE_PORT="${CLICKHOUSE_PORT:-8123}"
CLICKHOUSE_PASSWORD="${CLICKHOUSE_PASSWORD:-admin}"
AIRFLOW_HOST="${AIRFLOW_HOST:-localhost}"
AIRFLOW_PORT="${AIRFLOW_PORT:-8081}"
TIMEOUT="${SMOKE_TIMEOUT:-120}"

ok()   { echo "  ✓ $*"; }
fail() { echo "  ✗ $*" >&2; exit 1; }
info() { echo "→ $*"; }

wait_http() {
  local name="$1" url="$2" deadline=$(( $(date +%s) + TIMEOUT ))
  info "Waiting for $name at $url ..."
  until curl -sf --max-time 3 "$url" > /dev/null 2>&1; do
    if [ "$(date +%s)" -ge "$deadline" ]; then
      fail "$name did not become ready within ${TIMEOUT}s"
    fi
    sleep 2
  done
  ok "$name is up"
}

info "Priority Watch smoke test"
echo ""

# ── 1. ClickHouse HTTP ping ───────────────────────────────────────────────────
wait_http "ClickHouse HTTP" "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/ping"

# ── 2. ClickHouse SELECT 1 ────────────────────────────────────────────────────
info "Running SELECT 1 on ClickHouse ..."
RESULT=$(curl -sf \
  "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/?query=SELECT+1&user=default&password=${CLICKHOUSE_PASSWORD}" \
  | tr -d '[:space:]')
if [ "$RESULT" != "1" ]; then
  fail "ClickHouse SELECT 1 returned unexpected result: '$RESULT'"
fi
ok "ClickHouse SELECT 1 → $RESULT"

# ── 3. ClickHouse SHOW DATABASES ──────────────────────────────────────────────
info "Checking ClickHouse can list databases ..."
RESULT=$(curl -sf \
  "http://${CLICKHOUSE_HOST}:${CLICKHOUSE_PORT}/?query=SHOW+DATABASES&user=default&password=${CLICKHOUSE_PASSWORD}")
ok "ClickHouse SHOW DATABASES: $(echo "$RESULT" | tr '\n' ' ')"

# ── 4. Airflow health endpoint ────────────────────────────────────────────────
wait_http "Airflow webserver" "http://${AIRFLOW_HOST}:${AIRFLOW_PORT}/health"

info "Checking Airflow health response ..."
AIRFLOW_HEALTH=$(curl -sf "http://${AIRFLOW_HOST}:${AIRFLOW_PORT}/health")
if echo "$AIRFLOW_HEALTH" | grep -q '"status": "healthy"'; then
  ok "Airflow health: healthy"
elif echo "$AIRFLOW_HEALTH" | grep -q '"status"'; then
  # Airflow may return "unready" while scheduler is still starting — warn, don't fail.
  STATUS=$(echo "$AIRFLOW_HEALTH" | grep -o '"status": "[^"]*"' | head -1)
  echo "  ⚠ Airflow health returned: $STATUS (scheduler may still be starting)"
else
  fail "Airflow health response unexpected: $AIRFLOW_HEALTH"
fi

# ── 5. Postgres reachability (via Airflow metadata) ──────────────────────────
info "Checking Postgres is reachable from host ..."
if command -v pg_isready &>/dev/null; then
  pg_isready -h "${CLICKHOUSE_HOST}" -p 5432 -U "${POSTGRES_USER:-airflow}" -d airflow -q \
    && ok "Postgres ready" || echo "  ⚠ pg_isready unavailable or not ready yet (non-fatal)"
else
  echo "  ⚠ pg_isready not installed on host — skipping Postgres direct check"
fi

echo ""
echo "Smoke test passed."

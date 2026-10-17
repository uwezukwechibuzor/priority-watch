#!/usr/bin/env bash
# dev.sh — start the full Priority Watch stack locally without Docker.
#
# What it starts:
#   1. ClickHouse  (native binary or Homebrew)
#   2. pw-ingester migrate  (schema migrations, once)
#   3. pw-api      (REST API on :8080)
#   4. pw-ingester follow  (live block follower)
#   5. apps/web    (Vite dev server on :5173)
#
# Usage:
#   ./scripts/dev.sh            # start everything
#   ./scripts/dev.sh --no-follow  # skip the live follower (saves CPU)
#   ./scripts/dev.sh --dbt      # run dbt refresh after startup
#
# Requirements:
#   clickhouse   brew install clickhouse   OR  curl https://clickhouse.com/ | sh
#   go           https://go.dev/dl/
#   pnpm         npm install -g pnpm
#   dbt          pip install dbt-clickhouse   (only if using --dbt)
#
# Press Ctrl+C to stop all services cleanly.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# ── colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

log()  { echo -e "${CYAN}[dev]${RESET} $*"; }
ok()   { echo -e "${GREEN}[dev]${RESET} $*"; }
warn() { echo -e "${YELLOW}[dev]${RESET} $*"; }
die()  { echo -e "${RED}[dev] ERROR:${RESET} $*" >&2; exit 1; }

# ── flags ─────────────────────────────────────────────────────────────────────
RUN_FOLLOW=true
RUN_DBT=false
for arg in "$@"; do
  case $arg in
    --no-follow) RUN_FOLLOW=false ;;
    --dbt)       RUN_DBT=true ;;
    --help|-h)
      grep '^# ' "$0" | sed 's/^# //'
      exit 0 ;;
  esac
done

# ── load .env ─────────────────────────────────────────────────────────────────
if [[ -f "$ROOT/.env" ]]; then
  set -o allexport
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +o allexport
  ok "loaded .env"
else
  warn ".env not found — copying .env.example"
  cp "$ROOT/.env.example" "$ROOT/.env"
  warn "Edit .env and set ARBITRUM_RPC_URL / ARBITRUM_WS_URL, then re-run."
  exit 1
fi

# ── check required env vars ───────────────────────────────────────────────────
[[ -z "${ARBITRUM_RPC_URL:-}" || "$ARBITRUM_RPC_URL" == *"YOUR_KEY"* ]] && \
  die "ARBITRUM_RPC_URL is not set in .env — add your Alchemy/Infura key."

# ── check dependencies ────────────────────────────────────────────────────────
log "checking dependencies..."

check_cmd() {
  if ! command -v "$1" &>/dev/null; then
    die "'$1' not found. Install it: $2"
  fi
}

check_cmd go   "https://go.dev/dl/"
check_cmd pnpm "npm install -g pnpm"

# ClickHouse: support both Homebrew and the official single-binary install
if command -v clickhouse &>/dev/null; then
  CH_CMD="clickhouse server"
elif command -v clickhouse-server &>/dev/null; then
  CH_CMD="clickhouse-server"
else
  die "ClickHouse not found.\n  Install via Homebrew:  brew install clickhouse\n  Or official binary:    curl https://clickhouse.com/ | sh"
fi

ok "all dependencies found"

# ── PID tracking & cleanup ────────────────────────────────────────────────────
PIDS=()

cleanup() {
  echo ""
  log "shutting down all services..."
  for pid in "${PIDS[@]}"; do
    kill "$pid" 2>/dev/null || true
  done
  # Give processes a moment then force-kill any stragglers
  sleep 1
  for pid in "${PIDS[@]}"; do
    kill -9 "$pid" 2>/dev/null || true
  done
  ok "all services stopped. Goodbye."
}
trap cleanup EXIT INT TERM

# ── log directory ─────────────────────────────────────────────────────────────
LOG_DIR="$ROOT/.dev-logs"
mkdir -p "$LOG_DIR"

start_service() {
  local name="$1"; shift
  local logfile="$LOG_DIR/${name}.log"
  log "starting ${BOLD}${name}${RESET}  (log → .dev-logs/${name}.log)"
  "$@" >"$logfile" 2>&1 &
  PIDS+=($!)
}

# ── 1. ClickHouse ─────────────────────────────────────────────────────────────
log "checking if ClickHouse is already running..."
if curl -s "http://localhost:8123/ping" | grep -q "Ok"; then
  ok "ClickHouse already running on :8123"
else
  log "starting ClickHouse..."
  CH_DATA="$ROOT/.dev-clickhouse"
  mkdir -p "$CH_DATA"
  start_service "clickhouse" $CH_CMD \
    -- \
    --path="$CH_DATA" \
    --tcp_port=9000 \
    --http_port=8123 \
    --logger.console=1 \
    --logger.level=warning \
    --listen_host=127.0.0.1

  log "waiting for ClickHouse to be ready..."
  for i in $(seq 1 30); do
    if curl -s "http://localhost:8123/ping" | grep -q "Ok"; then
      ok "ClickHouse ready"
      break
    fi
    if [[ $i -eq 30 ]]; then
      die "ClickHouse did not start after 30s. Check .dev-logs/clickhouse.log"
    fi
    sleep 1
  done
fi

# ── 2. Build Go binaries ──────────────────────────────────────────────────────
log "building Go binaries..."
make build
ok "binaries built → bin/pw-ingester  bin/pw-api"

# ── 3. Run migrations ─────────────────────────────────────────────────────────
log "running ClickHouse migrations..."
"$ROOT/bin/pw-ingester" migrate
ok "migrations applied"

# ── 4. Start pw-api ───────────────────────────────────────────────────────────
start_service "pw-api" "$ROOT/bin/pw-api"
sleep 1
if curl -s "http://localhost:${API_PORT:-8080}/healthz" | grep -q "ok"; then
  ok "pw-api  ready  →  http://localhost:${API_PORT:-8080}"
  ok "        docs   →  http://localhost:${API_PORT:-8080}/docs"
else
  warn "pw-api started but /healthz not responding yet (check .dev-logs/pw-api.log)"
fi

# ── 5. Start live follower (optional) ─────────────────────────────────────────
if [[ "$RUN_FOLLOW" == "true" ]]; then
  start_service "pw-follower" "$ROOT/bin/pw-ingester" follow
  ok "pw-follower  running  (streaming live blocks from SQD)"
else
  warn "pw-follower skipped (--no-follow). Data will not update until you run: make follow"
fi

# ── 6. Install frontend deps if needed ───────────────────────────────────────
if [[ ! -d "$ROOT/apps/web/node_modules" ]]; then
  log "installing frontend dependencies (first run)..."
  (cd "$ROOT/apps/web" && pnpm install)
fi

# ── 7. Start Vite dev server ──────────────────────────────────────────────────
start_service "vite" bash -c "cd '$ROOT/apps/web' && pnpm dev"
sleep 2
ok "frontend  ready  →  http://localhost:5173"

# ── 8. Optional dbt refresh ───────────────────────────────────────────────────
if [[ "$RUN_DBT" == "true" ]]; then
  if ! command -v dbt &>/dev/null; then
    warn "dbt not found — skipping. Install: pip install dbt-clickhouse"
  else
    log "running dbt incremental refresh..."
    (cd "$ROOT/dbt" && dbt run \
      --profiles-dir . \
      --select marts.fct_blocks marts.agg_blocks_hourly marts.fct_treasury_daily \
      --target dev) \
      && ok "dbt refresh complete" \
      || warn "dbt refresh failed — check output above"
  fi
fi

# ── ready ─────────────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo -e "${GREEN}${BOLD}  Priority Watch running locally${RESET}"
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo -e "  Dashboard  →  ${CYAN}http://localhost:5173${RESET}"
echo -e "  API        →  ${CYAN}http://localhost:${API_PORT:-8080}/api/v1/blocks/kpis?range=7d${RESET}"
echo -e "  API docs   →  ${CYAN}http://localhost:${API_PORT:-8080}/docs${RESET}"
echo -e "  ClickHouse →  ${CYAN}http://localhost:8123${RESET}"
echo ""
echo -e "  Logs in    →  ${YELLOW}.dev-logs/${RESET}"
echo -e "  dbt        →  ${YELLOW}cd dbt && dbt run${RESET}  (run manually to refresh data)"
echo -e "  backfill   →  ${YELLOW}make backfill FROM=<block> TO=head${RESET}"
echo ""
echo -e "  Press ${BOLD}Ctrl+C${RESET} to stop all services."
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo ""

# ── tail logs ─────────────────────────────────────────────────────────────────
tail -f "$LOG_DIR"/*.log

#!/usr/bin/env bash
#
# Runs the whole platform: API and frontend together, one Ctrl-C to stop both.
#
#   ./app.sh                 mock data, no database (default)
#   ./app.sh --db            Postgres from DATABASE_URL; migrates, then serves
#   ./app.sh --db --seed     ... and replaces its contents with the demo fixture
#
# Environment:
#   BACKEND_PORT (8080)  FRONTEND_PORT (5173)
#   DATABASE_URL         required by --db
#   OLT_SECRETS_KEY      base64 32 bytes; enables device credential storage
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$ROOT/.logs"
BACKEND_PORT="${BACKEND_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"

USE_DB=0
SEED=0
for arg in "$@"; do
  case "$arg" in
    --db) USE_DB=1 ;;
    --seed) SEED=1 ;;
    -h|--help) sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

command -v go >/dev/null || { echo "go is required" >&2; exit 1; }
FRONTEND_CMD="npm run dev"
if command -v pnpm >/dev/null; then FRONTEND_CMD="pnpm dev"; fi
[ -d "$ROOT/frontend/node_modules" ] || { echo "run 'pnpm install' (or npm install) in frontend/ first" >&2; exit 1; }

mkdir -p "$LOG_DIR"

PIDS=()
cleanup() {
  echo
  echo "stopping…"
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# --- database ------------------------------------------------------------
BACKEND_ARGS=(-addr "127.0.0.1:${BACKEND_PORT}")
if [ "$USE_DB" = "1" ]; then
  [ -n "${DATABASE_URL:-}" ] || { echo "--db needs DATABASE_URL" >&2; exit 1; }
  MIGRATE_ARGS=(-dsn "$DATABASE_URL")
  [ "$SEED" = "1" ] && MIGRATE_ARGS+=(-seed)
  echo "→ migrating"
  go run ./backend/cmd/migrate "${MIGRATE_ARGS[@]}"
  BACKEND_ARGS+=(-dsn "$DATABASE_URL")
  export VITE_USE_MOCK_API=false
else
  echo "→ mock mode: no database, frontend serves fixtures"
  export VITE_USE_MOCK_API=true
fi

# --- backend -------------------------------------------------------------
echo "→ api on :$BACKEND_PORT"
(cd "$ROOT" && go run ./backend/cmd/api "${BACKEND_ARGS[@]}") >"$LOG_DIR/backend.log" 2>&1 &
PIDS+=($!)

for _ in $(seq 1 60); do
  if curl -sf "http://127.0.0.1:${BACKEND_PORT}/api/v1/health" >/dev/null 2>&1; then break; fi
  sleep 0.5
done
curl -sf "http://127.0.0.1:${BACKEND_PORT}/api/v1/health" >/dev/null \
  || { echo "api did not become healthy — see $LOG_DIR/backend.log" >&2; tail -20 "$LOG_DIR/backend.log" >&2; exit 1; }

# --- frontend ------------------------------------------------------------
echo "→ frontend on 127.0.0.1:$FRONTEND_PORT  (logs: $LOG_DIR/frontend.log)"
# --host 127.0.0.1: Vite binds "localhost", which can resolve to ::1 only, and
# then http://127.0.0.1:<port> answers nothing while the log says it is ready.
(cd "$ROOT/frontend" && $FRONTEND_CMD --host 127.0.0.1 --port "$FRONTEND_PORT") >"$LOG_DIR/frontend.log" 2>&1 &
PIDS+=($!)

cat <<INFO

  API      http://127.0.0.1:${BACKEND_PORT}/api/v1/health
  Frontend http://127.0.0.1:${FRONTEND_PORT}
  Mode     $( [ "$USE_DB" = "1" ] && echo "postgres" || echo "mock data" )
  Logs     $LOG_DIR

  Ctrl-C stops both.
INFO

# Wait for either process to exit, then clean up the other.
wait -n 2>/dev/null || wait

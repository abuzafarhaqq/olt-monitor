#!/usr/bin/env bash
# Run the Postgres integration tests against PGlite: a real PostgreSQL compiled
# to WebAssembly, no server required.
#
#   ./scripts/pglite-test.sh              # go test ./...
#   ./scripts/pglite-test.sh -v -run Test   # extra args go to go test
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PORT="${PGLITE_PORT:-5433}"
# simple_protocol: pgx's prepared-statement cache collides with pglite-socket's
# single shared backend. Real Postgres does not need it.
DSN="postgres://postgres@127.0.0.1:${PORT}/postgres?sslmode=disable&default_query_exec_mode=simple_protocol"

cd "$ROOT/tools/pglite"
if [ ! -d node_modules ]; then
  echo "installing PGlite…"
  pnpm install --silent 2>/dev/null || npm install --silent
fi

PGLITE_PORT="$PORT" node server.mjs >/tmp/pglite-$$.log 2>&1 &
SERVER_PID=$!
cleanup() {
  kill "$SERVER_PID" 2>/dev/null || true
  wait "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT

# Wait for the port, not for a fixed sleep.
for _ in $(seq 1 60); do
  if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then
    exec 3<&- 3>&-
    break
  fi
  sleep 0.5
done

if ! head -1 /tmp/pglite-$$.log | grep -q "listening"; then
  echo "PGlite did not start:" >&2
  cat /tmp/pglite-$$.log >&2
  exit 1
fi

cd "$ROOT"
TEST_DATABASE_URL="$DSN" go test ./... -count=1 "$@"

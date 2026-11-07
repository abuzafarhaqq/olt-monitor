#!/usr/bin/env bash
# Run the Postgres integration tests against PGlite: a real PostgreSQL compiled
# to WebAssembly, no server required.
#
#   ./scripts/pglite-test.sh              # go test ./...
#   ./scripts/pglite-test.sh -v -run Test   # extra args go to go test
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# A fixed port collides with a server left running by an interrupted run, which
# then answers these tests with a stale database. Default to a random free-ish
# port; PGLITE_PORT still overrides.
PORT="${PGLITE_PORT:-$((5500 + RANDOM % 300))}"
# simple_protocol: pgx's prepared-statement cache collides with pglite-socket's
# single shared backend. Real Postgres does not need it.
DSN="postgres://postgres@127.0.0.1:${PORT}/postgres?sslmode=disable&default_query_exec_mode=simple_protocol"

cd "$ROOT/tools/pglite"
if [ ! -d node_modules ]; then
  echo "installing PGlite…"
  pnpm install --silent 2>/dev/null || npm install --silent
fi

# Fail loudly if something else already owns the port: a stale server would
# answer the tests with its own database and produce confusing results.
if (exec 3<>"/dev/tcp/127.0.0.1/${PORT}") 2>/dev/null; then
  echo "port ${PORT} is already in use — stop that process or set PGLITE_PORT" >&2
  exit 1
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
# -p 1: the packages share one database, and the postgres package seeds
# (TRUNCATE). Run in parallel and one package wipes another's rows mid-test.
TEST_DATABASE_URL="$DSN" go test -p 1 ./... -count=1 "$@"

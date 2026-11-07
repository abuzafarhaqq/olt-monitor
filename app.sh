#!/usr/bin/env bash
#
# OLT Monitor — runs all three components: database, API, frontend.
#
#   ./app.sh                 mock data; no database involved
#   ./app.sh --db            start/find a database, migrate, serve real data
#   ./app.sh --db --seed     ... and replace its contents with the demo fixture
#   ./app.sh --install       install dependencies, then exit
#   ./app.sh --pglite        ignore DATABASE_URL and use PGlite anyway
#   ./app.sh --no-install    never install anything, fail if something is missing
#
# Database resolution for --db, in order:
#   1. DATABASE_URL is set            → use it
#   2. docker + db/docker-compose.yml → compose up -d
#   3. node available                 → PGlite (PostgreSQL in WASM, in memory)
#   4. nothing                        → stop with instructions
#
# Env: BACKEND_PORT (8080) FRONTEND_PORT (5173) DATABASE_URL OLT_SECRETS_KEY
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_DIR="$ROOT/.logs"
KEYS_DIR="$ROOT/.secrets"
BACKEND_PORT="${BACKEND_PORT:-8080}"
FRONTEND_PORT="${FRONTEND_PORT:-5173}"

WANT_DB=0
WANT_SEED=0
WANT_INSTALL_ONLY=0
ALLOW_INSTALL=1
FORCE_PGLITE=0

for arg in "$@"; do
  case "$arg" in
    --db) WANT_DB=1 ;;
    --seed) WANT_SEED=1 ;;
    --install) WANT_INSTALL_ONLY=1 ;;
    --pglite) FORCE_PGLITE=1 ;;
    --no-install) ALLOW_INSTALL=0 ;;
    -h|--help) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

# --- output ---------------------------------------------------------------
step() { printf '\033[1;34m▸\033[0m %s\n' "$*"; }
ok()   { printf '\033[1;32m✓\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m!\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m✗\033[0m %s\n' "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

# --- dependency installation ---------------------------------------------
apt_install() {
  local pkg="$1"
  [ "$ALLOW_INSTALL" = "1" ] || die "$pkg is missing and --no-install was given"
  have apt-get || die "$pkg is missing and there is no apt-get to install it"
  [ "$(id -u)" = "0" ] || die "$pkg is missing; install it (needs root) or use --no-install after installing"
  warn "$pkg not found — installing with apt-get"
  DEBIAN_FRONTEND=noninteractive apt-get update -qq >/dev/null
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "$pkg" >/dev/null
}

ensure_go() {
  if ! have go; then
    apt_install golang-go
  fi
  local version; version="$(go version | awk '{print $3}')"
  ok "go $version"
}

ensure_node() {
  if ! have node; then
    apt_install nodejs
    have npm || apt_install npm
  fi
  local major; major="$(node -p 'process.versions.node.split(".")[0]')"
  [ "$major" -ge 18 ] || die "node $major is too old; Vite 6 needs 18+"
  ok "node $(node -p 'process.versions.node')"
}

package_manager() {
  if have pnpm; then echo pnpm; elif have npm; then echo npm; else
    if [ "$ALLOW_INSTALL" = "1" ] && have npm; then echo npm; else die "no pnpm or npm"; fi
  fi
}

ensure_frontend_deps() {
  local pm; pm="$(package_manager)"
  if [ ! -d "$ROOT/frontend/node_modules" ]; then
    step "installing frontend dependencies ($pm)"
    (cd "$ROOT/frontend" && $pm install --silent) || die "frontend install failed"
  fi
  ok "frontend dependencies present"
}

# A go.work file above the checkout (commonly ~/go.work) makes every go command
# in this single-module repo fail with "module ... is not one of the workspace
# modules". Neutralise it for this script; GOWORK=off is set explicitly in the
# environment first if the caller wants to keep it.
neutralise_workspace() {
  local work; work="$(go env GOWORK)"
  if [ -n "$work" ]; then
    warn "ignoring the Go workspace at $work (this repo is a single module)"
    warn "to stop seeing this: rm -f \"$work\" \"$work.sum\"   — or run with GOWORK=off"
    export GOWORK=off
  fi
}

ensure_go_deps() {
  step "resolving Go modules"
  (cd "$ROOT" && go mod download) || die "go mod download failed"
  ok "go modules resolved"
}

if [ "$WANT_INSTALL_ONLY" = "1" ]; then
  step "installing dependencies only"
  ensure_go
  neutralise_workspace
  ensure_node
  ensure_go_deps
  ensure_frontend_deps
  ok "everything installed — run ./app.sh to start"
  exit 0
fi

# --- secret key -----------------------------------------------------------
# Device credentials are encrypted with a key that must survive restarts and
# must never enter git. Generated once, kept out of the repo, 0600.
ensure_secret_key() {
  if [ -n "${OLT_SECRETS_KEY:-}" ]; then
    ok "using OLT_SECRETS_KEY from the environment"
    return
  fi
  mkdir -p "$KEYS_DIR"; chmod 700 "$KEYS_DIR"
  local file="$KEYS_DIR/olt_secrets_key"
  if [ ! -f "$file" ]; then
    head -c 32 /dev/urandom | base64 | tr -d '\n' >"$file"
    chmod 600 "$file"
    warn "generated a new device-credential key at $file — back it up, losing it makes stored OLT passwords unreadable"
  fi
  OLT_SECRETS_KEY="$(cat "$file")"
  export OLT_SECRETS_KEY
}

# --- database -------------------------------------------------------------
PG_PID=""
DSN_MODE="none"

# Connects and applies migrations. Applying them here is a side effect, but an
# idempotent one — and it means "the database is reachable" and "the schema is
# current" are proven by the same call.
db_ready() {
  (cd "$ROOT" && go run ./backend/cmd/migrate -dsn "$1" >/dev/null 2>&1)
}

resolve_database() {
  if [ "$FORCE_PGLITE" != "1" ] && [ -n "${DATABASE_URL:-}" ]; then
    step "checking DATABASE_URL"
    if ! db_ready "$DATABASE_URL"; then
      die "DATABASE_URL is set but nothing answers there:
       ${DATABASE_URL}

     pick one:
       • start that database (pg_ctlcluster, docker, brew services…)
       • unset DATABASE_URL      → this script starts PGlite instead
       • ./app.sh --db --pglite  → ignore DATABASE_URL for this run"
    fi
    DSN_MODE="external"
    return
  fi

  if have docker && [ -f "$ROOT/db/docker-compose.yml" ]; then
    step "starting Postgres with docker compose"
    (cd "$ROOT/db" && docker compose up -d) || die "docker compose failed"
    DATABASE_URL="postgres://oltmon:oltmon@localhost:5432/oltmon?sslmode=disable"
    for _ in $(seq 1 30); do
      db_ready "$DATABASE_URL" && break
      sleep 1
    done
    DSN_MODE="docker"
    return
  fi

  if have node; then
    step "no external database — starting PGlite (PostgreSQL in WASM)"
    local port=$((5600 + RANDOM % 300))
    if [ ! -d "$ROOT/tools/pglite/node_modules" ]; then
      (cd "$ROOT/tools/pglite" && "$(package_manager)" install --silent) || die "could not install PGlite"
    fi
    (cd "$ROOT/tools/pglite" && PGLITE_PORT="$port" node server.mjs) >"$LOG_DIR/pglite.log" 2>&1 &
    PG_PID=$!
    DATABASE_URL="postgres://postgres@127.0.0.1:${port}/postgres?sslmode=disable&default_query_exec_mode=simple_protocol"
    for _ in $(seq 1 40); do
      (exec 3<>"/dev/tcp/127.0.0.1/${port}") 2>/dev/null && { exec 3<&- 3>&-; break; }
      sleep 0.5
    done
    db_ready "$DATABASE_URL" || die "PGlite did not become reachable — see $LOG_DIR/pglite.log"
    DSN_MODE="pglite"
    warn "PGlite keeps data in memory: everything is gone when this script stops"
    return
  fi

  die "no database available: set DATABASE_URL, install docker, or install node for PGlite"
}

# --- lifecycle ------------------------------------------------------------
PIDS=()
cleanup() {
  echo
  step "stopping"
  for pid in "${PIDS[@]:-}"; do kill "$pid" 2>/dev/null || true; done
  [ -n "$PG_PID" ] && kill "$PG_PID" 2>/dev/null || true
  wait 2>/dev/null || true
  echo "stopped"
}
trap cleanup EXIT INT TERM

mkdir -p "$LOG_DIR"

# --- steps ----------------------------------------------------------------
ensure_go
neutralise_workspace
ensure_node
ensure_go_deps
ensure_frontend_deps

BACKEND_ARGS=(-addr "127.0.0.1:${BACKEND_PORT}")

if [ "$WANT_SEED" = "1" ] && [ "$WANT_DB" = "0" ]; then
  warn "--seed only applies to --db; ignored"
  WANT_SEED=0
fi

if [ "$WANT_DB" = "1" ]; then
  ensure_secret_key
  resolve_database

  MIGRATE_ARGS=(-dsn "$DATABASE_URL")
  [ "$WANT_SEED" = "1" ] && MIGRATE_ARGS+=(-seed)
  step "applying migrations${WANT_SEED:+ and demo fixture}"
  (cd "$ROOT" && go run ./backend/cmd/migrate "${MIGRATE_ARGS[@]}") || die "migration failed"

  BACKEND_ARGS+=(-dsn "$DATABASE_URL")
  export VITE_USE_MOCK_API=false
else
  step "mock mode: the frontend serves fixtures and the API uses its in-memory data"
  export VITE_USE_MOCK_API=true
  # cmd/api defaults its -dsn from DATABASE_URL, so an exported (or stale) DSN
  # would drag mock mode into Postgres and fail the health check. Say it
  # explicitly: no database here.
  BACKEND_ARGS+=(-dsn "")
fi

step "starting the API on :${BACKEND_PORT}"
(cd "$ROOT" && go run ./backend/cmd/api "${BACKEND_ARGS[@]}") >"$LOG_DIR/backend.log" 2>&1 &
PIDS+=($!)

for _ in $(seq 1 90); do
  curl -sf "http://127.0.0.1:${BACKEND_PORT}/api/v1/health" >/dev/null 2>&1 && break
  sleep 0.5
done
curl -sf "http://127.0.0.1:${BACKEND_PORT}/api/v1/health" >/dev/null \
  || { tail -20 "$LOG_DIR/backend.log" >&2; die "the API did not become healthy"; }

step "starting the frontend on :${FRONTEND_PORT}"
(cd "$ROOT/frontend" && "$(package_manager)" dev --host 127.0.0.1 --port "$FRONTEND_PORT") >"$LOG_DIR/frontend.log" 2>&1 &
PIDS+=($!)

for _ in $(seq 1 60); do
  curl -sf "http://127.0.0.1:${FRONTEND_PORT}/" >/dev/null 2>&1 && break
  sleep 0.5
done

if [ "$WANT_DB" = "1" ]; then
  DB_STATE="$DSN_MODE"
  SIGNIN='The database has no accounts yet. Create one:
           go run ./backend/cmd/userctl create -username admin \
             -name "Your Name" -email you@example.net -role administrator'
else
  DB_STATE="not used (mock mode)"
  SIGNIN='Sign in: admin | engineer | operator | viewer — password "demo"'
fi

cat <<INFO

  ┌─────────────┬──────────────────────────────────────────────┐
  │ database    │ ${DB_STATE}
  │ api         │ http://127.0.0.1:${BACKEND_PORT}/api/v1/health
  │ frontend    │ http://127.0.0.1:${FRONTEND_PORT}
  │ logs        │ ${LOG_DIR}
  └─────────────┴──────────────────────────────────────────────┘

  ${SIGNIN}

  Ctrl-C stops everything.

INFO

wait -n 2>/dev/null || wait

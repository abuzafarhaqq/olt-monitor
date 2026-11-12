#!/usr/bin/env bash
#
# OLT Monitor — runs all three components: database, API, frontend.
#
#   ./app.sh                 mock data; no database involved
#   ./app.sh --db            start/find a database, migrate, serve real data
#   ./app.sh --db --seed     ... and replace its contents with the demo fixture
#   ./app.sh --install       install dependencies, then exit
#   ./app.sh --lan           listen on 0.0.0.0 so other machines can reach it
#   ./app.sh --stop          kill this app's processes that this shell can see
#   ./app.sh --mock          ignore any configured database
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
FORCE_MOCK=0
LAN=0
BIND_HOST="127.0.0.1"
CONFIG_FILE="$ROOT/.secrets/app.env"

for arg in "$@"; do
  case "$arg" in
    --db) WANT_DB=1 ;;
    --seed) WANT_SEED=1 ;;
    --install) WANT_INSTALL_ONLY=1 ;;
    --mock) FORCE_MOCK=1 ;;
    --lan) LAN=1 ;;
    --stop) STOP=1 ;;
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

if [ "${STOP:-0}" = "1" ]; then
  # Only what this namespace can see: a process started from another shell or
  # another proot is invisible here, and no /proc/net access is available to
  # find it by port without root.
  for pattern in '\.bin/api' 'exe/api' 'vite'; do
    pkill -f "$pattern" 2>/dev/null && echo "stopped: $pattern" || true
  done
  echo "done (processes outside this shell's view are not affected)"
  exit 0
fi

# --- configuration ------------------------------------------------------
# .secrets/app.env is the single place for the DSN and the device passwords.
# 0600, gitignored, sourced here so ./app.sh alone knows how to reach the
# database. Marked with set -a so every child process inherits it.
if [ -f "$CONFIG_FILE" ]; then
  chmod 600 "$CONFIG_FILE" 2>/dev/null || true
  set -a
  # shellcheck disable=SC1090
  . "$CONFIG_FILE"
  set +a
  step "configuration loaded from ${CONFIG_FILE#$ROOT/}"
fi

# A configured database means "run against it" — no --db flag needed.
if [ "${DATABASE_URL:-}" != "" ] && [ "$FORCE_MOCK" != "1" ]; then
  WANT_DB=1
fi
if [ "$FORCE_MOCK" = "1" ]; then
  WANT_DB=0
  DATABASE_URL=""
fi

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
  local modules="$ROOT/frontend/node_modules"
  local lock="$ROOT/frontend/pnpm-lock.yaml"

  # pnpm stamps node_modules/.modules.yaml when it installs; npm uses
  # .package-lock.json. Comparing times against the lockfile is what catches a
  # dependency added in a commit you just pulled — "node_modules exists" does
  # not, which is how @xterm/addon-fit crashed the dev server on first load.
  local marker="$modules/.modules.yaml"
  [ -f "$marker" ] || marker="$modules/.package-lock.json"
  [ -f "$lock" ] || lock="$ROOT/frontend/package-lock.json"

  local stale=0
  [ -d "$modules" ] || stale=1
  [ "$stale" = "0" ] && [ -f "$marker" ] && [ -f "$lock" ] && [ "$lock" -nt "$marker" ] && stale=1
  [ "$stale" = "0" ] && [ -f "$marker" ] && [ "$ROOT/frontend/package.json" -nt "$marker" ] && stale=1

  if [ "$stale" = "1" ]; then
    step "installing frontend dependencies ($pm) — lockfile or package.json changed"
    (cd "$ROOT/frontend" && $pm install) || die "frontend install failed"
  fi
  ok "frontend dependencies match the lockfile"
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

  # A Postgres that is already listening beats starting anything. Falling
  # through to PGlite here would look exactly like data loss: the app would come
  # up with an empty database while the real one sits on 5432.
  if command -v pg_isready >/dev/null 2>&1 && pg_isready -h 127.0.0.1 -p 5432 >/dev/null 2>&1; then
    die "a Postgres is already accepting connections on 127.0.0.1:5432.

     Point the script at it instead of starting another:
       export DATABASE_URL=\"postgres://oltmon:<password>@127.0.0.1:5432/oltmon?sslmode=disable\"
       ./app.sh --db
     (add that export to ~/.bashrc so it survives a new shell)"
  fi

  # Docker only when the compose plugin actually exists. The CLI alone is common
  # on Termux and \"docker compose\" then fails with a flag error.
  if have docker && [ -f "$ROOT/db/docker-compose.yml" ]; then
    if (cd "$ROOT/db" && docker compose version >/dev/null 2>&1); then
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
    if command -v docker-compose >/dev/null 2>&1; then
      step "starting Postgres with docker-compose"
      (cd "$ROOT/db" && docker-compose up -d) || die "docker-compose failed"
      DATABASE_URL="postgres://oltmon:oltmon@localhost:5432/oltmon?sslmode=disable"
      for _ in $(seq 1 30); do
        db_ready "$DATABASE_URL" && break
        sleep 1
      done
      DSN_MODE="docker-compose"
      return
    fi
    warn "docker is installed but the compose plugin is not — skipping it"
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

  mkdir -p "$ROOT/.secrets"; chmod 700 "$ROOT/.secrets"
  if [ ! -f "$CONFIG_FILE" ]; then
    cat >"$CONFIG_FILE" <<'TEMPLATE'
# Sourced by app.sh on every start. Keep this file 0600 and never commit it.
#
# The database the app should use. Leave commented to run on fixture data.
# DATABASE_URL="postgres://oltmon:YOUR_PASSWORD@127.0.0.1:5432/oltmon?sslmode=disable"
#
# Device passwords for the character-mode CLI and privileged mode. The SSH
# password itself lives encrypted in olt_credentials, not here.
# OLT_CLI_PASSWORD=
# OLT_ENABLE_PASSWORD=
#
# Set to "false" to let the Refresh button show the plan but never dial.
# OLT_ALLOW_COLLECT=true
TEMPLATE
    chmod 600 "$CONFIG_FILE"
    die "no database configured.

     I created ${CONFIG_FILE#$ROOT/} — fill in DATABASE_URL there (one line),
     or start Postgres and re-run with --db, or use ./app.sh --mock for fixtures."
  fi
  die "no database available: set DATABASE_URL in ${CONFIG_FILE#$ROOT/}, install docker, or install node for PGlite"
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

if [ "$LAN" = "1" ]; then
  BIND_HOST="0.0.0.0"
  # Only the UI is exposed. The API stays on loopback and the Vite proxy reaches
  # it from the same process: one less port open on the network, and the session
  # cookie still works because the browser only ever talks to :5173.
  warn "--lan: the UI is reachable from every host on this network, over plain HTTP (API stays on 127.0.0.1)"
fi

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
  if [ "$WANT_SEED" = "1" ]; then
    step "applying migrations and replacing the demo fixture"
  else
    step "applying migrations"
  fi
  (cd "$ROOT" && go run ./backend/cmd/migrate "${MIGRATE_ARGS[@]}") || die "migration failed"

  BACKEND_ARGS+=(-dsn "$DATABASE_URL")
  # Absolute: the API worker inherits this to verify device keys, so the
  # Refresh button in the UI can dial the same devices the CLI does.
  export OLT_KNOWN_HOSTS="$ROOT/.secrets/known_hosts"
  export VITE_USE_MOCK_API=false
else
  step "mock mode: the frontend serves fixtures and the API uses its in-memory data"
  export VITE_USE_MOCK_API=true
  # cmd/api defaults its -dsn from DATABASE_URL, so an exported (or stale) DSN
  # would drag mock mode into Postgres and fail the health check. Say it
  # explicitly: no database here.
  BACKEND_ARGS+=(-dsn "")
fi

port_taken() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null && { exec 3<&- 3>&-; return 0; }; return 1; }

for port in "$BACKEND_PORT" "$FRONTEND_PORT"; do
  if port_taken "$port"; then
    die "port ${port} is already in use — another instance is still running.

     stop it:
       pkill -f 'exe/api'      # an API started by `go run` runs as .../exe/api
       pkill -f '\.bin/api'    # …or as .bin/api, once this script starts it
       pkill -f vite
     or move this one: BACKEND_PORT=$((BACKEND_PORT+10)) FRONTEND_PORT=$((FRONTEND_PORT+10)) ./app.sh"
  fi
done

step "building the API"
mkdir -p "$ROOT/.bin"
BUILD_STAMP="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
if ! (cd "$ROOT" && go build -ldflags "-X main.buildTime=${BUILD_STAMP}" -o "$ROOT/.bin/api" ./backend/cmd/api); then
  die "the API did not compile"
fi

step "starting the API on :${BACKEND_PORT} (build ${BUILD_STAMP})"
"$ROOT/.bin/api" "${BACKEND_ARGS[@]}" >"$LOG_DIR/backend.log" 2>&1 &
PIDS+=($!)

for _ in $(seq 1 90); do
  if curl -sf "http://127.0.0.1:${BACKEND_PORT}/api/v1/health" >/dev/null 2>&1; then break; fi
  sleep 0.5
done
curl -sf "http://127.0.0.1:${BACKEND_PORT}/api/v1/health" >/dev/null \
  || { tail -20 "$LOG_DIR/backend.log" >&2; die "the API did not become healthy"; }

step "starting the frontend on :${FRONTEND_PORT}"
# Start vite directly when it is installed. Going through `pnpm dev` adds pnpm's
# own work first — lockfile supply-chain verification, script hooks — which
# writes to the terminal, ignores the log redirect and can take a minute. The
# dev server is all this script needs.
if [ -x "$ROOT/frontend/node_modules/.bin/vite" ]; then
  (cd "$ROOT/frontend" && exec ./node_modules/.bin/vite --host "$BIND_HOST" --port "$FRONTEND_PORT") \
    >"$LOG_DIR/frontend.log" 2>&1 &
else
  (cd "$ROOT/frontend" && exec "$(package_manager)" dev --host "$BIND_HOST" --port "$FRONTEND_PORT") \
    >"$LOG_DIR/frontend.log" 2>&1 &
fi
PIDS+=($!)

FRONTEND_UP=0
for _ in $(seq 1 120); do
  # `cmd && break` as the last statement of a loop body aborts under set -e;
  # the frontend taking a few seconds to boot is normal, not a failure.
  if curl -sf "http://127.0.0.1:${FRONTEND_PORT}/" >/dev/null 2>&1; then
    FRONTEND_UP=1
    break
  fi
  sleep 0.5
done
if [ "$FRONTEND_UP" = "0" ]; then
  tail -15 "$LOG_DIR/frontend.log" >&2
  die "the frontend did not answer on :${FRONTEND_PORT}"
fi

# The address another machine should use, when we know it.
LAN_IP=""
if [ "$LAN" = "1" ]; then
  # `X="$(missing-cmd)"` returns 127, and set -e aborts the script there —
  # which killed a healthy API two steps before the banner. Every probe here is
  # best-effort and must end with || true.
  LAN_IP="$(ip -4 addr show 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | grep -v '^127\.' | head -1 || true)"
  [ -n "$LAN_IP" ] || LAN_IP="$(ifconfig 2>/dev/null | awk '/inet /{print $2}' | grep -v '^127\.' | head -1 || true)"
  [ -n "$LAN_IP" ] || LAN_IP="$(getprop dhcp.wlan0.ipaddress 2>/dev/null || true)"
  if [ -z "$LAN_IP" ]; then
    warn "could not detect this machine's IP — look it up in Android Wi-Fi settings"
  fi
fi

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
  │ frontend    │ http://127.0.0.1:${FRONTEND_PORT}$( [ -n "$LAN_IP" ] && printf '\n  │ from LAN    │ http://%s:%s' "$LAN_IP" "$FRONTEND_PORT" )
  │ logs        │ ${LOG_DIR}
  └─────────────┴──────────────────────────────────────────────┘

  ${SIGNIN}

  Ctrl-C stops everything.$( [ "$LAN" = "1" ] && printf '\n\n  Anyone on this network can reach the login page. There is no TLS and no\n  rate limiting yet: use --lan on a trusted network only.' )

INFO

wait -n 2>/dev/null || wait

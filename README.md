# OLT Monitor

Monorepo for the OLT diagnostics and monitoring platform: React frontend, Go
backend, PostgreSQL schema.

```
olt-monitor/
├── frontend/     React 18 + TypeScript + Vite (SPA)
├── backend/      Go 1.22 API, stores, auth
├── db/           schema migrations, docker-compose, embedded SQL package
├── scripts/      pglite-test.sh — integration tests without a database server
└── app.sh        start API + frontend together
```

## Quick start

```bash
cd frontend && pnpm install && cd ..   # once
./app.sh                               # mock data, no database
```

Then open http://127.0.0.1:5173. Mock sign-in: `admin` / `engineer` /
`operator` / `viewer`, password `demo`.

With a database:

```bash
export DATABASE_URL="postgres://oltmon:oltmon@localhost:5432/oltmon?sslmode=disable"
export OLT_SECRETS_KEY="$(head -c 32 /dev/urandom | base64)"   # device credentials
./app.sh --db --seed                   # migrate, load the demo fixture, serve
```

`--seed` **replaces** the database contents. Never point it at production.

## What runs where

| Path | Purpose |
|---|---|
| `frontend/src/features/*` | one directory per screen |
| `frontend/src/services/data.ts` | mock or API data source, chosen by `VITE_USE_MOCK_API` |
| `backend/cmd/api` | HTTP server (`-addr`, `-dsn`) |
| `backend/cmd/userctl` | create / passwd / list accounts before there is a user API |
| `backend/cmd/migrate` | apply embedded migrations, optional `-seed` |
| `db/migrations/*.sql` | the schema; embedded into the binaries via `db/embed.go` |

`app.sh` sets `VITE_USE_MOCK_API=false` automatically when `--db` is used, so the
frontend and backend agree on the mode without a second switch.

## Tests

```bash
make test           # unit + handler tests (no database needed)
make test-pglite    # the same suite against PostgreSQL compiled to WASM
make test-db        # against a real Postgres in TEST_DATABASE_URL
```

`test-pglite` needs Node and installs PGlite into `tools/pglite` on first run.

## Environment

| Variable | Used by | Meaning |
|---|---|---|
| `DATABASE_URL` | backend, `app.sh --db` | Postgres DSN; empty = in-memory fixture |
| `OLT_SECRETS_KEY` | backend | base64 32 bytes; enables encrypted device credentials |
| `VITE_USE_MOCK_API` | frontend build | `false` talks to the API |
| `BACKEND_PORT` / `FRONTEND_PORT` | `app.sh` | defaults 8080 / 5173 |

## Status

Working end to end: overview, inventory, PON explorer, ONU list/detail, device
create/edit with write-only credentials, sessions and role enforcement.

Still fixture-backed: incidents, reports, optical diagnostics, collection health,
and the ONU optical-history / raw-evidence tabs.

No command is ever sent to an OLT by this system yet — the collector does not
exist, and the browser never speaks SSH or Telnet.

# OLT Monitor — backend

Go 1.22, `net/http`, pgx v5. Serves the contract the frontend already declares
in `src/services/api.ts`.

## Run

```bash
go run ./cmd/api                       # in-memory fixture, no database needed
go run ./cmd/api -dsn "$DATABASE_URL"  # Postgres
```

```bash
curl -s localhost:8080/api/v1/health
curl -s localhost:8080/api/v1/dashboard/overview | head -c 400
```

## Database

```bash
createdb oltmon && createdb oltmon_test
go run ./cmd/migrate -dsn "postgres://oltmon:oltmon@localhost:5432/oltmon?sslmode=disable" -seed
```

Or with Docker:

```bash
docker compose up -d          # postgres 16 on :5432, user/pass/db = oltmon
```

The seed replaces the contents of the target database with the demo fixture and
mirrors `internal/store/memory` — same numbers either way.

## Test

```bash
go vet ./... && go test ./...        # unit + SQL parse; database tests skip

# needs a scratch database; the seed wipes it
TEST_DATABASE_URL="postgres://oltmon:oltmon@localhost:5432/oltmon_test?sslmode=disable" go test ./...
```

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/health` | liveness |
| GET | `/api/v1/dashboard/overview` | everything the Overview page needs |
| GET | `/api/v1/olts` | inventory |
| GET | `/api/v1/olts/{id}` | one device |
| GET | `/api/v1/olts/{id}/pons` | PON ports |
| GET | `/api/v1/pons/{id}/onus` | ONUs on a port |
| GET | `/api/v1/onus/{id}` | one ONU |
| GET | `/api/v1/onus/{id}/optical` | RX history, gaps preserved as null |
| GET | `/api/v1/events` | raw events, duplicates included |
| GET | `/api/v1/incidents` · `/api/v1/incidents/{id}` | cases |
| GET | `/api/v1/collection/jobs` | latest run per device and job type |
| GET | `/api/v1/stream` | SSE: `ready` frame, then heartbeats |

Errors are `{"error":{"code":"not_found","message":"..."}}`. A missing row is
404; everything else is a 500 that does not leak internals.

## Authentication

Users live in Postgres (`migrations/0002_users.sql`). Three secrets, three
treatments: password → **argon2id** PHC string; security answer → argon2id;
TOTP secret → **AES-256-GCM** (`internal/crypto`), because it must be
reproduced to check a code.

Sessions are random 32-byte tokens; the database stores only the SHA-256 hash,
so a dump cannot be replayed as a login. Cookie is `HttpOnly`, `SameSite=Strict`,
`Secure` whenever the request arrived over TLS, 12-hour expiry.

```bash
# bootstrap the first account (password is prompted, never a flag)
DATABASE_URL=... userctl create -username alice -name "Alice Rahman" \
  -email alice@example.net -role administrator
DATABASE_URL=... userctl passwd -username alice    # also revokes her sessions
DATABASE_URL=... userctl list
```

```
POST /api/v1/auth/login   {username, password}  → 200 + session cookie
GET  /api/v1/auth/me                            → 401 without a session
POST /api/v1/auth/logout                        → 204, row revoked
POST /api/v1/auth/password-reset {email}        → 204 always, audit row only
```

An unknown username, a wrong password and a disabled account all answer the
same 401 with the same body — and an unknown username still spends the argon2
time, so the response clock does not enumerate users.

Not yet: TOTP verification, lockout after repeated failures, password reset,
user-management endpoints, and the login page in the frontend.

## Rules the code enforces

- **A failed collection never writes ONU state.** `onus.status` only changes on
  an authoritative device answer; the fixture proves it (an unreachable OLT
  yields `unknown`, never `offline`).
- **Unknown is a real bucket.** `OnuCounts` has three counters.
- **Missing is null, not zero.** `rx_power_dbm` and every optional measurement is
  nullable; the history keeps its gaps.
- **Two clocks.** `DeviceTimestamps` carries the device's claim, our ingestion
  time and a `quality` flag, because the first VSOL EPON reports May 2000.
- **Duplicates are data.** No unique constraint on event content; two captures of
  one event are two rows with different evidence references.
- **Thresholds live in the database.** `olt_thresholds` is per
  `(vendor, technology)`. Low-optical counts join against it, so an uncalibrated
  pair contributes nothing instead of a guess.

## Verification status

The development sandbox cannot start a native PostgreSQL (seccomp denies
`shmget`; there is no `/dev/shm`, so `shm_open` returns ENOENT). That is an
environment limit, not a code limit — and it is worked around rather than
ignored: **PGlite runs the real PostgreSQL core compiled to WebAssembly**, and
the integration suite runs against it.

```bash
./scripts/pglite-test.sh        # or: make test-pglite
```

| Check | Status |
|---|---|
| `go build ./...`, `go vet ./...` | passing |
| Handler tests against the in-memory store | passing |
| Migration executed by PostgreSQL | **yes — PostgreSQL 18.6 (PGlite)** |
| Every query executed, columns verified | **yes** |
| Seed, counts, thresholds, duplicates, null gaps | **yes** |
| Against a native server (roles, extensions, TCP auth) | **not yet — run it once on your host** |

What PGlite does not prove: a native server's authentication, roles,
extensions, or concurrency behaviour. `pglite-socket` multiplexes one backend,
so the DSN sets `default_query_exec_mode=simple_protocol` — pgx's prepared
statement cache collides with the shared backend. A real server needs no such
flag.

## Layout

```
cmd/api/                 main, flags (-addr, -dsn), graceful shutdown
cmd/migrate/             apply embedded migrations, optional -seed
internal/api/            routes, middleware, JSON helpers, SSE
internal/domain/         types; JSON names match the frontend contract
internal/store/          Store interface
internal/store/memory/   in-memory fixture
internal/store/postgres/ pgx implementation, migrations runner, seed
migrations/              0001_init.sql, embedded into the binaries
```

## Not built yet

- Collection worker (`cmd/collector`) and vendor adapters: SSH session, prompt
  handling, parsers, capability discovery.
- Write endpoints: incident acknowledge/note/resolve, device management, settings.
- Authentication and permission enforcement. The frontend has the role model; the
  API enforces nothing yet.
- SSE fan-out: the stream emits heartbeats only.
- Availability history: `trend()` is built from the current population, not from
  stored samples.

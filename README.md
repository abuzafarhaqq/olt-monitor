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

## Verification status — read this before trusting the SQL

The sandbox this was written in **cannot run PostgreSQL**: Android's seccomp
filter denies `shmget` (*Function not implemented*) and there is no `/dev/shm`
for POSIX shared memory (`shm_open` → ENOENT). The server binaries install, but
`initdb` cannot bootstrap a cluster.

What that means for this code:

| Check | Status |
|---|---|
| `go build ./...`, `go vet ./...` | passing |
| Handler tests against the in-memory store | passing |
| Every runtime query parsed with a PostgreSQL grammar | passing (`TestQueriesParse`) |
| Migration SQL executed | **never — needs a real database** |
| Queries executed, column names verified | **never — needs a real database** |

`TestQueriesParse` proves grammar, not semantics: a wrong column name parses and
fails at runtime. Run the integration suite once against a real database before
believing any of it:

```bash
TEST_DATABASE_URL=... go test ./internal/store/postgres/ -v
```

Those tests are skipped, not faked, when the variable is absent.

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

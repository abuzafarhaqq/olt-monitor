# OLT Monitor — backend

Go 1.22, standard library HTTP, no runtime dependencies yet. Serves the contract
the frontend already declares in `src/services/api.ts`.

## Run

```bash
go run ./cmd/api -addr :8080
curl -s localhost:8080/api/v1/health
curl -s localhost:8080/api/v1/dashboard/overview | head -c 400
```

Storage is an in-memory fixture today (`internal/store/memory`). The Postgres
implementation and the collection worker are the next increments — see
"Not built yet" below.

## Test

```bash
go vet ./...
go test ./...
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
| GET | `/api/v1/collection/jobs` | job results |
| GET | `/api/v1/stream` | SSE: `ready` frame, then heartbeats |

Errors are `{"error":{"code":"not_found","message":"..."}}`. A missing row is
404; everything else is a 500 that does not leak internals.

## Rules the code enforces

- **A failed collection never writes ONU state.** `onus.status` is only updated
  from an authoritative device answer; the fixture proves it (an unreachable OLT
  yields `unknown`, never `offline`).
- **Unknown is a real bucket.** `OnuCounts` has three counters, and the payment
  for merging them is a wrong outage report.
- **Missing is null, not zero.** `rxPowerDbm` and every optional measurement is a
  pointer; JSON emits `null` and the history keeps its gaps.
- **Two clocks.** `DeviceTimestamps` carries the device's claim, our ingestion
  time and a `quality` flag, because the first VSOL EPON reports May 2000.
- **Duplicates are data.** `events` has no `UNIQUE` on content; two captures of
  one event are two rows with different evidence references.
- **Identifiers only.** Handlers take ids, never SQL or CLI from the client.

## Layout

```
cmd/api/                 main, flags, graceful shutdown
internal/api/            routes, middleware, JSON helpers
internal/domain/         types; JSON names match the frontend contract
internal/store/          Store interface + memory implementation
migrations/0001_init.sql schema (enum types, evidence, measurements, audit)
```

## Not built yet

- `internal/store/postgres` — the pgx implementation of `Store`. Writing SQL that
  has never executed against a database would be guesswork; it lands with a real
  Postgres to test against.
- The collection worker (`cmd/collector`) and vendor adapters — SSH session,
  prompt handling, parsers, capability discovery.
- Write endpoints: incident acknowledge/note/resolve, device management, settings.
- Authentication and permission enforcement (the frontend has the role model;
  the API must enforce it, and nothing here does yet).
- SSE event fan-out: the stream currently emits heartbeats only.

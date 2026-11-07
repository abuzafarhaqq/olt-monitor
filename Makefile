.PHONY: build run test test-db test-pglite migrate seed fmt vet frontend

DATABASE_URL ?= postgres://oltmon:oltmon@localhost:5432/oltmon?sslmode=disable
TEST_DATABASE_URL ?= postgres://oltmon:oltmon@localhost:5432/oltmon_test?sslmode=disable

build:
	go build ./...

run:
	go run ./backend/cmd/api -addr :8080 -dsn "$(DATABASE_URL)"

test:
	go test ./...

# Integration suite against a real Postgres written in SQL. Needs the database
# to exist; the seed wipes it, so point TEST_DATABASE_URL at a scratch database.
test-db:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./... -count=1 -v

# Same suite against PostgreSQL compiled to WASM — no server, no Docker.
test-pglite:
	./scripts/pglite-test.sh

migrate:
	go run ./backend/cmd/migrate -dsn "$(DATABASE_URL)"

seed:
	go run ./backend/cmd/migrate -dsn "$(DATABASE_URL)" -seed

frontend:
	cd frontend && $(if $(shell command -v pnpm),pnpm dev,npm run dev)

fmt:
	gofmt -w .

vet:
	go vet ./...

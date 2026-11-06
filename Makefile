.PHONY: build run test test-db migrate seed fmt vet

DATABASE_URL ?= postgres://oltmon:oltmon@localhost:5432/oltmon?sslmode=disable
TEST_DATABASE_URL ?= postgres://oltmon:oltmon@localhost:5432/oltmon_test?sslmode=disable

build:
	go build ./...

run:
	go run ./cmd/api -addr :8080 -dsn "$(DATABASE_URL)"

test:
	go test ./...

# Runs the Postgres integration tests. Needs a reachable database; the seed
# replaces its contents, so point TEST_DATABASE_URL at a scratch database.
test-db:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./... -count=1 -v

migrate:
	go run ./cmd/migrate -dsn "$(DATABASE_URL)"

seed:
	go run ./cmd/migrate -dsn "$(DATABASE_URL)" -seed

fmt:
	gofmt -w .

vet:
	go vet ./...

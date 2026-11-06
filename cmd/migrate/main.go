// Command migrate applies the embedded SQL migrations and optionally seeds the
// demo fixture.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/abuzafarhaqq/olt-monitor-backend/internal/store/postgres"
	"github.com/abuzafarhaqq/olt-monitor-backend/migrations"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "Postgres connection string")
	seed := flag.Bool("seed", false, "replace the database contents with the demo fixture")
	flag.Parse()

	if *dsn == "" {
		slog.Error("no DSN: pass -dsn or set DATABASE_URL")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	store, err := postgres.New(ctx, *dsn)
	if err != nil {
		slog.Error("connect", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := postgres.Migrate(ctx, store.Pool(), migrations.FS); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")

	if *seed {
		if err := postgres.Seed(ctx, store.Pool()); err != nil {
			slog.Error("seed", "error", err)
			os.Exit(1)
		}
		slog.Info("demo fixture loaded")
	}
}

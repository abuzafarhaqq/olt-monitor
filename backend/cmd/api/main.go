// Command api serves the read API from the in-memory store.
//
// The Postgres store arrives next; until then this binary exists so the
// frontend can flip USE_MOCK_API and find out what the real contract feels
// like — including latency, status codes and error bodies.
package main

import (
	"context"
	"errors"
	"flag"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/config"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/api"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collecting"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/crypto"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/live"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/secrets"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/memory"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
	"github.com/abuzafarhaqq/olt-monitor/db"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// buildTime is set by app.sh with -ldflags. Without it, "is the server running
// the code I just pulled?" is unanswerable from the outside.
var buildTime = "dev"

func main() {
	// Same file app.sh sources, so a CLI does not need a hand-exported DSN.
	_, _ = config.LoadEnvFile()

	addr := flag.String("addr", ":8080", "listen address")
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "Postgres DSN; empty = in-memory fixture")
	logLevel := flag.String("log-level", "info", "debug|info|warn|error")
	flag.Parse()

	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		slog.Error("invalid log level", "value", *logLevel)
		os.Exit(2)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	// Storage is a flag, not a rebuild: the same binary serves the fixture or a
	// real database. Only Postgres can hold users, so only Postgres enables login.
	var handler http.Handler
	if *dsn == "" {
		logger.Warn("no DSN given; serving the in-memory fixture without authentication")
		handler = api.NewServer(memory.New(), logger).WithBuild(buildTime).Routes()
	} else {
		pg, err := postgres.New(context.Background(), *dsn)
		if err != nil {
			logger.Error("connect to postgres", "error", err)
			os.Exit(1)
		}
		defer pg.Close()
		logger.Info("connected to postgres")

		// Migrations belong to the binary, not to a shell: a user who only ever
		// opens the browser must never be asked to run one.
		migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if err := postgres.Migrate(migrateCtx, pg.Pool(), db.FS); err != nil {
			migrateCancel()
			logger.Error("migrations failed", "error", err)
			os.Exit(1)
		}
		migrateCancel()
		logger.Info("schema up to date")
		server := api.NewServer(pg, logger).WithBuild(buildTime).WithAuth(pg).WithAudit(pg).WithDevices(pg).WithUsers(pg)

		var creds *secrets.Service

		// Credential storage needs a key. Without one the endpoints answer 503
		// rather than storing device passwords in the clear.
		key, keyErr := crypto.KeyFromEnv("OLT_SECRETS_KEY")
		if keyErr != nil {
			logger.Warn("credential storage disabled", "reason", keyErr)
		} else {
			ring, ringErr := crypto.NewKeyring(1, key)
			if ringErr != nil {
				logger.Error("credential key invalid", "error", ringErr)
				os.Exit(1)
			}
			creds = secrets.New(ring, pg)
			server = server.WithSecrets(creds)
			logger.Info("credential storage enabled")
		}

		// Collection service: the UI's "refresh" button ends here.
		collectSvc := &collecting.Service{
			Pool:    pg.Pool(),
			CLIPass: os.Getenv("OLT_CLI_PASSWORD"),
			// Same account the CLI uses, so a device without a stored credential
			// still collects — and says so in the stream.
			FallbackUser: os.Getenv("OLT_USER"),
			FallbackPass: os.Getenv("OLT_PASSWORD"),
			EnablePass:   os.Getenv("OLT_ENABLE_PASSWORD"),
			AllowRun:     os.Getenv("OLT_ALLOW_COLLECT") != "false",
			// Whole-run budget and per-command budget: a 25,000-ONU estate
			// prints megabytes and takes minutes per table.
			Timeout:        45 * time.Minute,
			CommandTimeout: 5 * time.Minute,
			// Same file the CLI records keys into. Without it every collection
			// refuses to dial, which is what the first UI refresh hit.
			KnownHosts: firstNonEmpty(os.Getenv("OLT_KNOWN_HOSTS"), ".secrets/known_hosts"),
		}
		if creds != nil {
			collectSvc.Secrets = creds
		}
		hub := live.NewHub()
		collectSvc.Hub = hub
		collectSvc.Keys = pg
		collectSvc.Commands = pg
		server = server.WithCollector(collectSvc).WithLogs(pg).WithHub(hub).WithCommandList(pg)

		handler = server.Routes()
	}

	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		// No WriteTimeout: it would kill long-lived SSE connections.
		IdleTimeout: 120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("api listening", "addr", *addr, "build", buildTime)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

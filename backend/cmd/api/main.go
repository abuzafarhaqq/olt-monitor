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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/api"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collecting"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/crypto"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/secrets"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/memory"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
)

func main() {
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
		handler = api.NewServer(memory.New(), logger).Routes()
	} else {
		pg, err := postgres.New(context.Background(), *dsn)
		if err != nil {
			logger.Error("connect to postgres", "error", err)
			os.Exit(1)
		}
		defer pg.Close()
		logger.Info("connected to postgres")
		server := api.NewServer(pg, logger).WithAuth(pg).WithAudit(pg).WithDevices(pg).WithUsers(pg)

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
			Pool:       pg.Pool(),
			CLIPass:    os.Getenv("OLT_CLI_PASSWORD"),
			EnablePass: os.Getenv("OLT_ENABLE_PASSWORD"),
			AllowRun:   os.Getenv("OLT_ALLOW_COLLECT") != "false",
			Timeout:    90 * time.Second,
		}
		if creds != nil {
			collectSvc.Secrets = creds
		}
		server = server.WithCollector(collectSvc)

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
		logger.Info("api listening", "addr", *addr)
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

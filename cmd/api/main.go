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

	"github.com/abuzafarhaqq/olt-monitor-backend/internal/api"
	"github.com/abuzafarhaqq/olt-monitor-backend/internal/store/memory"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	logLevel := flag.String("log-level", "info", "debug|info|warn|error")
	flag.Parse()

	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		slog.Error("invalid log level", "value", *logLevel)
		os.Exit(2)
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	server := &http.Server{
		Addr:              *addr,
		Handler:           api.NewServer(memory.New(), logger).Routes(),
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

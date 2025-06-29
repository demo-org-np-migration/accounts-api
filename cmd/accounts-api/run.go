package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	kitlog "github.com/demo-org-np-migration/cauri-go-kit/log"

	httpapi "github.com/demo-org-np-migration/accounts-api/internal/http"
	"github.com/demo-org-np-migration/accounts-api/internal/store"
)

func run() {
	serviceName := getEnv("SERVICE_NAME", "accounts-api")
	env := getEnv("ENV", "staging")
	port := getEnv("PORT", "8080")
	keycloakIssuer := os.Getenv("KEYCLOAK_ISSUER")
	databaseURL := os.Getenv("DATABASE_URL")

	logger := kitlog.New(serviceName, env)

	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	if keycloakIssuer == "" {
		logger.Error("KEYCLOAK_ISSUER is required")
		os.Exit(1)
	}

	pool, err := connectWithRetry(logger, databaseURL, 10, 2*time.Second)
	if err != nil {
		logger.Error("could not connect to ledger", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	pg := store.NewPostgres(pool)
	router := httpapi.NewRouter(pg, logger, keycloakIssuer)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		logger.Info("listening", "port", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	logger.Info("shutting down")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// connectWithRetry retries store.Connect up to attempts times. Without
// this, accounts-api starting alongside a Postgres that's still doing
// initdb (docker-compose's depends_on doesn't wait for readiness, and
// neither does a freshly-rolled pod in k8s without an initContainer) means
// the very first Ping fails and the process exits before Postgres ever
// gets a chance to come up.
func connectWithRetry(logger *slog.Logger, databaseURL string, attempts int, backoff time.Duration) (*pgxpool.Pool, error) {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, err := store.Connect(ctx, databaseURL)
		cancel()
		if err == nil {
			return pool, nil
		}
		lastErr = err
		logger.Warn("ledger not reachable yet, retrying", "attempt", i, "attempts", attempts, "err", err)
		time.Sleep(backoff)
	}
	return nil, lastErr
}

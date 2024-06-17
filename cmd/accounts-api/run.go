package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := store.Connect(ctx, databaseURL)
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

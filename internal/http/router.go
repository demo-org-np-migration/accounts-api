// Package httpapi wires accounts-api's routes: chi for the mux,
// cauri-go-kit/auth for the Bearer guard on every /v1 route (plus
// RequireRole("service") on freeze), cauri-go-kit/metrics for /metrics.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/demo-org-np-migration/cauri-go-kit/auth"

	"github.com/demo-org-np-migration/accounts-api/internal/metrics"
	"github.com/demo-org-np-migration/accounts-api/internal/store"
)

// NewRouter builds the full mux: /health and /metrics are public, every
// /v1 route requires a valid Bearer token for keycloakIssuer, and freeze
// additionally requires the "service" role.
func NewRouter(st store.Store, logger *slog.Logger, keycloakIssuer string) http.Handler {
	h := &Handlers{Store: st, Logger: logger}

	r := chi.NewRouter()
	r.Use(requestLogging(logger))
	r.Use(metrics.Middleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/v1", func(r chi.Router) {
		r.Use(auth.Middleware(keycloakIssuer))

		r.Get("/accounts/{id}", h.GetAccount)
		r.Get("/accounts/{id}/statement", h.Statement)
		r.Get("/customers/{id}/accounts", h.ListAccountsByCustomer)

		r.With(auth.RequireRole("service")).Post("/accounts/{id}/freeze", h.FreezeAccount)
	})

	return r
}

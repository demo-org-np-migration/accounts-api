package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/demo-org-np-migration/accounts-api/internal/store"
)

// Handlers holds the dependencies every route needs. It's built once in
// router.go and its methods are wired up as chi handlers.
type Handlers struct {
	Store  store.Store
	Logger *slog.Logger
}

// GetAccount handles GET /v1/accounts/{id}.
func (h *Handlers) GetAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	account, err := h.Store.GetAccount(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_not_found", "no account with that id")
		return
	}
	if err != nil {
		h.Logger.Error("get account failed", "account_id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load the account")
		return
	}

	writeJSON(w, http.StatusOK, account)
}

// ListAccountsByCustomer handles GET /v1/customers/{id}/accounts.
func (h *Handlers) ListAccountsByCustomer(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")

	accounts, err := h.Store.ListAccountsByCustomer(r.Context(), customerID)
	if err != nil {
		h.Logger.Error("list accounts by customer failed", "customer_id", customerID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load the accounts")
		return
	}

	writeJSON(w, http.StatusOK, accounts)
}

// FreezeAccount handles POST /v1/accounts/{id}/freeze. The router only
// reaches this handler for requests that already passed
// auth.RequireRole("service").
func (h *Handlers) FreezeAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	account, err := h.Store.FreezeAccount(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "account_not_found", "no account with that id")
		return
	}
	if err != nil {
		h.Logger.Error("freeze account failed", "account_id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not freeze the account")
		return
	}

	writeJSON(w, http.StatusOK, account)
}

// Statement handles GET /v1/accounts/{id}/statement?from=&to=. from/to are
// RFC3339 timestamps; missing from defaults to the epoch, missing to
// defaults to now, so "just give me everything" works without query params.
func (h *Handlers) Statement(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	from, err := parseTimeParam(r.URL.Query().Get("from"), time.Unix(0, 0).UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_from", "from must be RFC3339")
		return
	}
	to, err := parseTimeParam(r.URL.Query().Get("to"), time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_to", "to must be RFC3339")
		return
	}

	entries, err := h.Store.Statement(r.Context(), id, from, to)
	if err != nil {
		h.Logger.Error("statement failed", "account_id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load the statement")
		return
	}

	writeJSON(w, http.StatusOK, entries)
}

func parseTimeParam(raw string, fallback time.Time) (time.Time, error) {
	if raw == "" {
		return fallback, nil
	}
	return time.Parse(time.RFC3339, raw)
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{"code": code, "message": message},
	})
}

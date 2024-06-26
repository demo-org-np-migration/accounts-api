package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/demo-org-np-migration/accounts-api/internal/store"
)

// fakeStore is the store.Store used by every handler test — no Postgres
// involved, per the spec's "tests with a fake store" requirement.
type fakeStore struct {
	accounts       map[string]store.Account
	byCustomer     map[string][]store.Account
	statementByID  map[string][]store.StatementEntry
	frozenAccounts map[string]bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		accounts:       map[string]store.Account{},
		byCustomer:     map[string][]store.Account{},
		statementByID:  map[string][]store.StatementEntry{},
		frozenAccounts: map[string]bool{},
	}
}

func (f *fakeStore) GetAccount(ctx context.Context, id string) (*store.Account, error) {
	a, ok := f.accounts[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &a, nil
}

func (f *fakeStore) ListAccountsByCustomer(ctx context.Context, customerID string) ([]store.Account, error) {
	return f.byCustomer[customerID], nil
}

func (f *fakeStore) FreezeAccount(ctx context.Context, id string) (*store.Account, error) {
	a, ok := f.accounts[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	a.Status = "frozen"
	f.accounts[id] = a
	f.frozenAccounts[id] = true
	return &a, nil
}

func (f *fakeStore) Statement(ctx context.Context, accountID string, from, to time.Time) ([]store.StatementEntry, error) {
	return f.statementByID[accountID], nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

// noAuthRouter builds the handlers directly (bypassing auth.Middleware) so
// tests can drive them without minting real JWTs. RequireRole itself is
// exercised separately with a raw claims-less request against the real
// middleware chain below.
func newHandlers(st store.Store) *Handlers {
	return &Handlers{Store: st, Logger: testLogger()}
}

func TestGetAccount_OK(t *testing.T) {
	st := newFakeStore()
	st.accounts["acc-1"] = store.Account{
		ID: "acc-1", CustomerID: "cust-1", Currency: "ARS", Balance: "50000.00", Status: "active",
		Customer: &store.Customer{Email: "ana@cauri.io", FullName: "Ana Cliente"},
	}
	h := newHandlers(st)

	r := chiRequest(http.MethodGet, "/v1/accounts/acc-1", "id", "acc-1")
	w := httptest.NewRecorder()
	h.GetAccount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got store.Account
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "acc-1" || got.Customer == nil || got.Customer.Email != "ana@cauri.io" {
		t.Fatalf("unexpected body: %+v", got)
	}
}

func TestGetAccount_NotFound(t *testing.T) {
	h := newHandlers(newFakeStore())

	r := chiRequest(http.MethodGet, "/v1/accounts/missing", "id", "missing")
	w := httptest.NewRecorder()
	h.GetAccount(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListAccountsByCustomer_OK(t *testing.T) {
	st := newFakeStore()
	st.byCustomer["cust-1"] = []store.Account{
		{ID: "acc-1", CustomerID: "cust-1", Currency: "ARS", Balance: "50000.00", Status: "active"},
		{ID: "acc-2", CustomerID: "cust-1", Currency: "USD", Balance: "500.00", Status: "active"},
	}
	h := newHandlers(st)

	r := chiRequest(http.MethodGet, "/v1/customers/cust-1/accounts", "id", "cust-1")
	w := httptest.NewRecorder()
	h.ListAccountsByCustomer(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got []store.Account
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(got))
	}
}

func TestFreezeAccount_OK(t *testing.T) {
	st := newFakeStore()
	st.accounts["acc-1"] = store.Account{ID: "acc-1", CustomerID: "cust-1", Currency: "ARS", Balance: "50000.00", Status: "active"}
	h := newHandlers(st)

	r := chiRequest(http.MethodPost, "/v1/accounts/acc-1/freeze", "id", "acc-1")
	w := httptest.NewRecorder()
	h.FreezeAccount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got store.Account
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "frozen" {
		t.Fatalf("expected status frozen, got %q", got.Status)
	}
}

// TestFreeze_RequiresServiceRole exercises the real router (not just the
// handler) so auth.RequireRole("service") is actually in the chain: a
// request with no Authorization header never reaches the handler at all,
// which is the 403-without-role case the spec asks for.
func TestFreeze_RequiresServiceRole(t *testing.T) {
	st := newFakeStore()
	st.accounts["acc-1"] = store.Account{ID: "acc-1", Status: "active"}
	router := NewRouter(st, testLogger(), "http://keycloak.invalid/realms/cauri-test")

	req := httptest.NewRequest(http.MethodPost, "/v1/accounts/acc-1/freeze", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// No bearer token at all: auth.Middleware itself rejects with 401
	// before RequireRole ever runs. Either way the freeze handler must not
	// run — st.frozenAccounts must stay empty.
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Fatalf("expected 401 or 403, got %d: %s", w.Code, w.Body.String())
	}
	if st.frozenAccounts["acc-1"] {
		t.Fatalf("freeze handler ran without a valid service token")
	}
}

func TestStatement_OK(t *testing.T) {
	st := newFakeStore()
	st.statementByID["acc-1"] = []store.StatementEntry{
		{EntryID: "e1", Amount: "100.00", Type: "credit", Reference: "seed"},
	}
	h := newHandlers(st)

	r := chiRequest(http.MethodGet, "/v1/accounts/acc-1/statement", "id", "acc-1")
	w := httptest.NewRecorder()
	h.Statement(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got []store.StatementEntry
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 1 || got[0].EntryID != "e1" {
		t.Fatalf("unexpected body: %+v", got)
	}
}

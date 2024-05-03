// Package store is accounts-api's data layer. It reads (and, for freeze,
// writes) straight against the `ledger` Postgres database — the one
// ledger-core owns — instead of going through ledger-core's API. See
// docs/adr/ADR-003-lectura-directa.md; this was meant to be temporary.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by any lookup that finds nothing, so handlers can
// map it to a 404 without inspecting driver-specific errors.
var ErrNotFound = errors.New("store: not found")

// Customer is the slice of `customers` that accounts-api's read model
// embeds under Account.Customer.
type Customer struct {
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

// Account mirrors `ledger.accounts`, joined with its owning customer for
// GET /v1/accounts/{id}.
type Account struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Currency   string    `json:"currency"`
	Balance    string    `json:"balance"`
	Status     string    `json:"status"`
	Customer   *Customer `json:"customer,omitempty"`
}

// StatementEntry mirrors one row of `ledger.ledger_entries`.
type StatementEntry struct {
	EntryID   string    `json:"entry_id"`
	Amount    string    `json:"amount"`
	Type      string    `json:"type"`
	Reference string    `json:"reference"`
	CreatedAt time.Time `json:"created_at"`
}

// Store is everything the HTTP layer needs from the data layer. Handlers
// are tested against a fake implementation; Postgres is the real one.
type Store interface {
	// GetAccount returns the account with its owning customer joined in.
	// ErrNotFound if no account has that id.
	GetAccount(ctx context.Context, id string) (*Account, error)

	// ListAccountsByCustomer returns every account belonging to customerID,
	// in no particular order. An empty slice, not an error, if there are none.
	ListAccountsByCustomer(ctx context.Context, customerID string) ([]Account, error)

	// FreezeAccount sets status='frozen' on the account and returns the
	// updated row. ErrNotFound if no account has that id.
	FreezeAccount(ctx context.Context, id string) (*Account, error)

	// Statement returns ledger_entries for accountID with created_at in
	// [from, to], ordered oldest first.
	Statement(ctx context.Context, accountID string, from, to time.Time) ([]StatementEntry, error)
}

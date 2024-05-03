package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is the real Store, backed by a connection pool to the `ledger`
// database. accounts-api doesn't own that schema or ship migrations for
// it — ledger-core does both (see ADR-003).
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres builds a Postgres store from an already-open pool.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

// Connect opens a pgxpool against databaseURL and pings it once so startup
// fails fast if `ledger` isn't reachable.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

const getAccountQuery = `
SELECT a.id, a.customer_id, a.currency, a.balance::text, a.status,
       c.email, c.full_name
FROM accounts a
JOIN customers c ON c.id = a.customer_id
WHERE a.id = $1
`

// GetAccount joins accounts with customers directly against the ledger
// database — accounts-api's original sin, kept alive by ADR-003.
func (p *Postgres) GetAccount(ctx context.Context, id string) (*Account, error) {
	row := p.pool.QueryRow(ctx, getAccountQuery, id)

	var a Account
	var c Customer
	err := row.Scan(&a.ID, &a.CustomerID, &a.Currency, &a.Balance, &a.Status, &c.Email, &c.FullName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Customer = &c
	return &a, nil
}

const listAccountsByCustomerQuery = `
SELECT id, customer_id, currency, balance::text, status
FROM accounts
WHERE customer_id = $1
`

// ListAccountsByCustomer backs GET /v1/customers/{id}/accounts. No
// customer join here — the caller already has the customer id.
func (p *Postgres) ListAccountsByCustomer(ctx context.Context, customerID string) ([]Account, error) {
	rows, err := p.pool.Query(ctx, listAccountsByCustomerQuery, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.CustomerID, &a.Currency, &a.Balance, &a.Status); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

const freezeAccountQuery = `
UPDATE accounts
SET status = 'frozen'
WHERE id = $1
RETURNING id, customer_id, currency, balance::text, status
`

// FreezeAccount is the other half of the defect ADR-003 signs off on: a
// direct UPDATE against ledger-core's table instead of a call to its API.
// Guarded upstream by auth.RequireRole("service").
func (p *Postgres) FreezeAccount(ctx context.Context, id string) (*Account, error) {
	row := p.pool.QueryRow(ctx, freezeAccountQuery, id)

	var a Account
	err := row.Scan(&a.ID, &a.CustomerID, &a.Currency, &a.Balance, &a.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

const statementQuery = `
SELECT id, amount::text, type, reference, created_at
FROM ledger_entries
WHERE account_id = $1 AND created_at >= $2 AND created_at <= $3
ORDER BY created_at
`

// Statement reads ledger_entries for the statement endpoint.
func (p *Postgres) Statement(ctx context.Context, accountID string, from, to time.Time) ([]StatementEntry, error) {
	rows, err := p.pool.Query(ctx, statementQuery, accountID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []StatementEntry{}
	for rows.Next() {
		var e StatementEntry
		if err := rows.Scan(&e.EntryID, &e.Amount, &e.Type, &e.Reference, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

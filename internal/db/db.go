// Package db provides pgx/v5 connection pool initialisation and RLS helpers.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates and validates a pgx connection pool.
func New(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return pool, nil
}

// WithOrgID executes fn inside a transaction with app.current_org_id set for RLS.
// The transaction is automatically rolled back on error and committed on success.
// orgID is passed as a parameter to set_config to prevent SQL injection.
func WithOrgID(ctx context.Context, pool *pgxpool.Pool, orgID string, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_org_id', $1, true)", orgID); err != nil {
		return fmt.Errorf("set org_id: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

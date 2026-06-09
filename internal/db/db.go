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

// withRLSVar runs fn in a transaction with a single Postgres GUC set via
// set_config so RLS policies that read it (e.g. current_setting('app.current_org_id'))
// see the scoped value. Both the variable name and its value are passed as
// parameters to set_config, so neither is string-interpolated into SQL.
// Private — callers should use the WithOrgID / WithUserID wrappers.
func withRLSVar(
	ctx context.Context,
	pool *pgxpool.Pool,
	varName, varValue string,
	fn func(tx pgx.Tx) error,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, "SELECT set_config($1, $2, true)", varName, varValue); err != nil {
		return fmt.Errorf("set %s: %w", varName, err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WithOrgID executes fn inside a transaction with app.current_org_id set for RLS.
// The transaction is automatically rolled back on error and committed on success.
// orgID is passed as a parameter to set_config to prevent SQL injection.
func WithOrgID(ctx context.Context, pool *pgxpool.Pool, orgID string, fn func(tx pgx.Tx) error) error {
	return withRLSVar(ctx, pool, "app.current_org_id", orgID, fn)
}

// WithUserID executes fn inside a transaction with app.current_user_id set for
// RLS. The sibling of WithOrgID, used by user-scoped tables (e.g.
// user_passkey_labels, migration 00054) whose tenant boundary is the user
// rather than the organisation.
//
// userID is passed as a parameter to set_config to prevent SQL injection.
func WithUserID(ctx context.Context, pool *pgxpool.Pool, userID string, fn func(tx pgx.Tx) error) error {
	return withRLSVar(ctx, pool, "app.current_user_id", userID, fn)
}

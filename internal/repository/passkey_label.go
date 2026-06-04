// Package repository — passkey_label.go is the pgx-backed persistence layer
// for the user_passkey_labels table (migration 00054). It mirrors the shape
// of the other repositories in this package (e.g. llm_provider.go, chunk.go)
// but is user-scoped rather than org-scoped: RLS reads
// app.current_user_id, set by db.WithUserID.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
)

// PasskeyLabel is the projected shape of one row of user_passkey_labels
// returned to callers. The wire-level JSON shape lives in the handler;
// this is the domain model the service layer trades in.
type PasskeyLabel struct {
	CredentialID string
	Label        string
	CreatedAt    time.Time
	LastUsedAt   *time.Time
}

// ErrPasskeyLabelNotFound signals that a write targeted no rows — typically
// because the credential is not owned by the caller (or has never had a
// label persisted). Service / handler layers map this to HTTP 404.
var ErrPasskeyLabelNotFound = errors.New("passkey label not found")

// PasskeyLabelRepository handles database operations for the
// user_passkey_labels table. Each method opens its own RLS-scoped tx via
// db.WithUserID — single-statement reads/writes don't benefit from sharing
// a transaction with anything else in the request, and keeping the tx
// hidden inside the repo lets the service layer stay free of pgx.
type PasskeyLabelRepository struct {
	pool *pgxpool.Pool
}

// NewPasskeyLabelRepository creates a new PasskeyLabelRepository.
func NewPasskeyLabelRepository(pool *pgxpool.Pool) *PasskeyLabelRepository {
	return &PasskeyLabelRepository{pool: pool}
}

// ListForUser returns every label row owned by userID, keyed by
// credential_id for a cheap join against the SuperTokens core list.
func (r *PasskeyLabelRepository) ListForUser(ctx context.Context, userID string) (map[string]PasskeyLabel, error) {
	out := make(map[string]PasskeyLabel)
	err := db.WithUserID(ctx, r.pool, userID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT credential_id, label, created_at, last_used_at
			 FROM user_passkey_labels
			 WHERE user_id = $1`, userID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				row        PasskeyLabel
				lastUsedAt *time.Time
			)
			if err := rows.Scan(&row.CredentialID, &row.Label, &row.CreatedAt, &lastUsedAt); err != nil {
				return err
			}
			row.LastUsedAt = lastUsedAt
			out[row.CredentialID] = row
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("PasskeyLabelRepository.ListForUser: %w", err)
	}
	return out, nil
}

// Upsert inserts a new row or updates the label on an existing one.
// Behaviour is intentionally "label only" — created_at is never touched on
// update, and last_used_at is owned by the SuperTokens hook path (set when
// the credential is used for sign-in).
func (r *PasskeyLabelRepository) Upsert(ctx context.Context, userID, credentialID, label string) error {
	err := db.WithUserID(ctx, r.pool, userID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO user_passkey_labels (user_id, credential_id, label)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (credential_id) DO UPDATE SET label = EXCLUDED.label`,
			userID, credentialID, label)
		return err
	})
	if err != nil {
		return fmt.Errorf("PasskeyLabelRepository.Upsert: %w", err)
	}
	return nil
}

// UpdateLabel atomically renames the label for an existing row owned by
// userID. It returns ErrPasskeyLabelNotFound when no row matches (either
// the credential never had a label persisted, or it belongs to another
// user — both are 404 from the caller's point of view). The single
// UPDATE is the ownership check: no pre-query is required.
func (r *PasskeyLabelRepository) UpdateLabel(ctx context.Context, userID, credentialID, label string) error {
	var rowsAffected int64
	err := db.WithUserID(ctx, r.pool, userID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE user_passkey_labels
			 SET label = $1
			 WHERE credential_id = $2 AND user_id = $3`,
			label, credentialID, userID)
		if err != nil {
			return err
		}
		rowsAffected = tag.RowsAffected()
		return nil
	})
	if err != nil {
		return fmt.Errorf("PasskeyLabelRepository.UpdateLabel: %w", err)
	}
	if rowsAffected == 0 {
		return ErrPasskeyLabelNotFound
	}
	return nil
}

// Delete removes a single label row. Callers should treat a missing row
// as success — the SuperTokens core is the source of truth for whether
// the credential itself was deleted; a missing label row just means the
// user never renamed the credential.
func (r *PasskeyLabelRepository) Delete(ctx context.Context, userID, credentialID string) error {
	err := db.WithUserID(ctx, r.pool, userID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`DELETE FROM user_passkey_labels WHERE user_id = $1 AND credential_id = $2`,
			userID, credentialID)
		return err
	})
	if err != nil {
		return fmt.Errorf("PasskeyLabelRepository.Delete: %w", err)
	}
	return nil
}

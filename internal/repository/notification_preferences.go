package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
)

// NotificationPreferencesRepository wraps access to user_notification_preferences
// (migration 00037). All mutating calls are RLS-scoped via db.WithOrgID.
type NotificationPreferencesRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationPreferencesRepository constructs the repo.
func NewNotificationPreferencesRepository(pool *pgxpool.Pool) *NotificationPreferencesRepository {
	return &NotificationPreferencesRepository{pool: pool}
}

// GetEmailSummariesEnabled returns the effective opt-in status for a user in a
// workspace. The result is the AND of:
//   - workspaces.email_summaries_enabled (admin-level master switch), AND
//   - user_notification_preferences.email_summaries_enabled (user opt-in).
//
// When no preference row exists, the default is FALSE (explicit opt-in).
func (r *NotificationPreferencesRepository) GetEmailSummariesEnabled(ctx context.Context, orgID, userID, workspaceID string) (bool, error) {
	const q = `
SELECT
    COALESCE((SELECT w.email_summaries_enabled FROM workspaces w WHERE w.id = $2 AND w.org_id = $3), FALSE)
    AND COALESCE(p.email_summaries_enabled, FALSE) AS enabled
FROM (SELECT 1) _
LEFT JOIN user_notification_preferences p
    ON p.user_id = $1 AND p.workspace_id = $2 AND p.org_id = $3
`
	var enabled bool
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, q, userID, workspaceID, orgID).Scan(&enabled)
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	return enabled, nil
}

// SetUserPreference upserts the user-level email-summaries toggle.
func (r *NotificationPreferencesRepository) SetUserPreference(ctx context.Context, orgID, userID, workspaceID string, enabled bool) error {
	const q = `
INSERT INTO user_notification_preferences (user_id, workspace_id, org_id, email_summaries_enabled, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (user_id, workspace_id) DO UPDATE
SET email_summaries_enabled = EXCLUDED.email_summaries_enabled, updated_at = NOW()
`
	return db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, q, userID, workspaceID, orgID, enabled)
		return err
	})
}

// SetWorkspacePreference flips the workspace-level master switch.
func (r *NotificationPreferencesRepository) SetWorkspacePreference(ctx context.Context, orgID, workspaceID string, enabled bool) error {
	const q = `UPDATE workspaces SET email_summaries_enabled = $3 WHERE id = $2 AND org_id = $1`
	return db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, q, orgID, workspaceID, enabled)
		return err
	})
}

// UnsubscribeByHash disables the email-summaries flag for a user across all
// workspaces where a preference row already exists. Used by the one-click
// unsubscribe endpoint, which has no notion of workspace scope.
func (r *NotificationPreferencesRepository) UnsubscribeAll(ctx context.Context, orgID, userID string) error {
	const q = `
UPDATE user_notification_preferences
SET email_summaries_enabled = FALSE, updated_at = NOW()
WHERE user_id = $1 AND org_id = $2
`
	return db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, q, userID, orgID)
		return err
	})
}

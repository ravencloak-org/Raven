package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ravencloak-org/Raven/internal/db"
)

// ErrConversationSessionNotFound is returned by SetSummary when the UPDATE
// matches zero rows (stale task id, wrong org, or RLS filter). Callers
// should treat this as a terminal failure for the Asynq task rather than
// silently proceed to send an email tied to a summary that was never
// persisted.
var ErrConversationSessionNotFound = errors.New("repository: conversation session not found")

// SetSummary persists the post-session recap text onto a conversation_sessions
// row. It is called by the email-summary Asynq handler once the AI worker
// returns a bullet list. RLS ensures the UPDATE only affects rows belonging
// to the caller's org.
//
// Returns ErrConversationSessionNotFound when the UPDATE affects zero rows.
// Exec returns nil for UPDATEs that match nothing, so without this check the
// worker would quietly claim success on a stale/forged session_id.
//
// Lives on ConversationRepository (introduced by #349) so we don't ship a
// parallel repository just for this column.
func (r *ConversationRepository) SetSummary(ctx context.Context, orgID, sessionID, summary string) error {
	const q = `UPDATE conversation_sessions SET summary = $1 WHERE id = $2 AND org_id = $3`
	return db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, q, summary, sessionID, orgID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrConversationSessionNotFound
		}
		return nil
	})
}

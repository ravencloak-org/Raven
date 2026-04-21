package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ravencloak-org/Raven/internal/db"
)

// SetSummary persists the post-session recap text onto a conversation_sessions
// row. It is called by the email-summary Asynq handler once the AI worker
// returns a bullet list. RLS ensures the UPDATE only affects rows belonging
// to the caller's org.
//
// Lives on ConversationRepository (introduced by #349) so we don't ship a
// parallel repository just for this column.
func (r *ConversationRepository) SetSummary(ctx context.Context, orgID, sessionID, summary string) error {
	const q = `UPDATE conversation_sessions SET summary = $1 WHERE id = $2 AND org_id = $3`
	return db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, q, summary, sessionID, orgID)
		return err
	})
}

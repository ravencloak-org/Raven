package account

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLRepo implements DSARRepo + RetentionRepo against a Postgres pool.
//
// ExportUser currently returns the user + org row only. Full enumeration
// of user-owned tables (messages, conversations, workspaces, sessions,
// etc.) is the responsibility of a follow-up commit once the canonical
// list is enumerated — see plan #3 spec §15 open question 1.
//
// HardDelete is intentionally NOT implemented yet. The cascade
// behaviour in a multi-tenant schema (delete org? just user?) is a
// product decision tracked in docs/superpowers/specs/2026-05-12-public-demo-deployment-design.md §6.
type SQLRepo struct {
	pool *pgxpool.Pool
}

// NewSQLRepo wraps a pgxpool for use as DSARRepo + RetentionRepo.
func NewSQLRepo(pool *pgxpool.Pool) *SQLRepo {
	return &SQLRepo{pool: pool}
}

// ExportUser returns user + org info for the authenticated user. Each
// user-owned domain table (messages, workspaces, etc.) must be added
// here as the schema is enumerated; see package doc.
func (r *SQLRepo) ExportUser(ctx context.Context, userID string) (UserExport, error) {
	out := UserExport{UserID: userID, Rows: map[string][]map[string]any{}}

	var email string
	var orgID string
	var displayName *string
	var createdAt time.Time
	err := r.pool.QueryRow(ctx,
		`SELECT email, display_name, org_id, created_at
		   FROM users WHERE id = $1`,
		userID,
	).Scan(&email, &displayName, &orgID, &createdAt)
	if err != nil {
		return out, err
	}
	out.Email = email

	userRow := map[string]any{
		"id":         userID,
		"email":      email,
		"org_id":     orgID,
		"created_at": createdAt,
	}
	if displayName != nil {
		userRow["display_name"] = *displayName
	}
	out.Rows["users"] = []map[string]any{userRow}

	// TODO(plan-3-task-40): enumerate user-owned domain tables here —
	// conversations, messages, workspaces, voice_sessions, etc. Spec §15.
	return out, nil
}

// ScheduleDelete inserts a row into scheduled_deletes with a 24h grace
// window. The drain worker (separate commit) reads from this table.
func (r *SQLRepo) ScheduleDelete(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO scheduled_deletes (user_id) VALUES ($1)
		 ON CONFLICT (user_id) DO NOTHING`,
		userID,
	)
	return err
}

// InactiveUsers returns active accounts whose last_login_at is older
// than `since`. The retention cron passes the warn cutoff (23 days
// ago); the purger applies the day-threshold policy.
func (r *SQLRepo) InactiveUsers(ctx context.Context, since time.Time) ([]InactiveUser, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, email, last_login_at
		   FROM users
		  WHERE status = 'active'
		    AND last_login_at IS NOT NULL
		    AND last_login_at < $1`,
		since,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []InactiveUser
	for rows.Next() {
		var u InactiveUser
		var last *time.Time
		if err := rows.Scan(&u.ID, &u.Email, &last); err != nil {
			return nil, err
		}
		if last != nil {
			u.LastActive = *last
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// MarkWarned records that a 7-day deletion warning was emailed.
// Idempotent — repeated calls do not duplicate the row.
func (r *SQLRepo) MarkWarned(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO account_purge_warnings (user_id) VALUES ($1)
		 ON CONFLICT (user_id) DO NOTHING`,
		userID,
	)
	return err
}

// ErrHardDeleteNotEnabled is returned by HardDelete until the cascade
// behaviour for a multi-tenant account purge is decided.
var ErrHardDeleteNotEnabled = errors.New(
	"account.SQLRepo.HardDelete is not enabled — see spec §6 for the " +
		"cascade decision (delete-org vs delete-user-only)",
)

// HardDelete is intentionally a no-op error until the cascade
// behaviour is finalised. The retention cron continues to MarkWarned
// and ScheduleDelete writes its row; only the final destructive step
// is gated. See package doc.
func (r *SQLRepo) HardDelete(_ context.Context, _ string) error {
	return ErrHardDeleteNotEnabled
}

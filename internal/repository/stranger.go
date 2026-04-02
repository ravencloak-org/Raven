package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// StrangerRepository handles database operations for anonymous chat users.
// All mutating operations run inside a pgx.Tx with org_id set for RLS.
type StrangerRepository struct {
	pool *pgxpool.Pool
}

// NewStrangerRepository creates a new StrangerRepository.
func NewStrangerRepository(pool *pgxpool.Pool) *StrangerRepository {
	return &StrangerRepository{pool: pool}
}

// Complete, atomically-declared SQL statements — no fragment concatenation.
const (
	sqlUpsertStranger = `
		INSERT INTO stranger_users (org_id, session_id, ip_address, user_agent, message_count)
		VALUES ($1, $2, $3::inet, $4, CASE WHEN $5 THEN 1 ELSE 0 END)
		ON CONFLICT (org_id, session_id) DO UPDATE SET
			last_active_at = NOW(),
			message_count  = stranger_users.message_count + CASE WHEN $5 THEN 1 ELSE 0 END,
			ip_address     = EXCLUDED.ip_address,
			user_agent     = EXCLUDED.user_agent
		RETURNING id, org_id, session_id,
			COALESCE(ip_address::text, '') AS ip_address,
			COALESCE(user_agent, '')       AS user_agent,
			status,
			COALESCE(block_reason, '')     AS block_reason,
			message_count, rate_limit_rpm,
			last_active_at, blocked_at,
			COALESCE(blocked_by::text, '') AS blocked_by,
			created_at, updated_at`

	sqlGetStrangerBySessionID = `
		SELECT id, org_id, session_id,
			COALESCE(ip_address::text, '') AS ip_address,
			COALESCE(user_agent, '')       AS user_agent,
			status,
			COALESCE(block_reason, '')     AS block_reason,
			message_count, rate_limit_rpm,
			last_active_at, blocked_at,
			COALESCE(blocked_by::text, '') AS blocked_by,
			created_at, updated_at
		FROM stranger_users
		WHERE org_id = $1 AND session_id = $2`

	sqlGetStrangerByID = `
		SELECT id, org_id, session_id,
			COALESCE(ip_address::text, '') AS ip_address,
			COALESCE(user_agent, '')       AS user_agent,
			status,
			COALESCE(block_reason, '')     AS block_reason,
			message_count, rate_limit_rpm,
			last_active_at, blocked_at,
			COALESCE(blocked_by::text, '') AS blocked_by,
			created_at, updated_at
		FROM stranger_users
		WHERE org_id = $1 AND id = $2`

	sqlListStrangersByStatus = `
		SELECT id, org_id, session_id,
			COALESCE(ip_address::text, '') AS ip_address,
			COALESCE(user_agent, '')       AS user_agent,
			status,
			COALESCE(block_reason, '')     AS block_reason,
			message_count, rate_limit_rpm,
			last_active_at, blocked_at,
			COALESCE(blocked_by::text, '') AS blocked_by,
			created_at, updated_at
		FROM stranger_users
		WHERE org_id = $1 AND status = $2
		ORDER BY last_active_at DESC LIMIT $3 OFFSET $4`

	sqlListStrangers = `
		SELECT id, org_id, session_id,
			COALESCE(ip_address::text, '') AS ip_address,
			COALESCE(user_agent, '')       AS user_agent,
			status,
			COALESCE(block_reason, '')     AS block_reason,
			message_count, rate_limit_rpm,
			last_active_at, blocked_at,
			COALESCE(blocked_by::text, '') AS blocked_by,
			created_at, updated_at
		FROM stranger_users
		WHERE org_id = $1
		ORDER BY last_active_at DESC LIMIT $2 OFFSET $3`
)

func scanStranger(row pgx.Row) (*model.StrangerUser, error) {
	var s model.StrangerUser
	var ipStr string
	var blockedBy string

	err := row.Scan(
		&s.ID, &s.OrgID, &s.SessionID,
		&ipStr,
		&s.UserAgent,
		&s.Status,
		&s.BlockReason,
		&s.MessageCount, &s.RateLimitRPM,
		&s.LastActiveAt, &s.BlockedAt,
		&blockedBy,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if ipStr != "" {
		s.IPAddress = &ipStr
	}
	if blockedBy != "" {
		s.BlockedBy = blockedBy
	}
	return &s, nil
}

// Upsert inserts a new stranger record or updates last_active_at and message_count
// on conflict with (org_id, session_id).
func (r *StrangerRepository) Upsert(ctx context.Context, tx pgx.Tx, orgID string, req model.UpsertStrangerRequest) (*model.StrangerUser, error) {
	var ipArg any
	if req.IPAddress != nil && *req.IPAddress != "" {
		ipArg = *req.IPAddress
	}

	row := tx.QueryRow(ctx, sqlUpsertStranger, orgID, req.SessionID, ipArg, req.UserAgent, req.IncrementCount)
	s, err := scanStranger(row)
	if err != nil {
		return nil, fmt.Errorf("StrangerRepository.Upsert: %w", err)
	}
	return s, nil
}

// GetBySessionID fetches a stranger record by session ID within an org.
func (r *StrangerRepository) GetBySessionID(ctx context.Context, tx pgx.Tx, orgID, sessionID string) (*model.StrangerUser, error) {
	row := tx.QueryRow(ctx, sqlGetStrangerBySessionID, orgID, sessionID)
	s, err := scanStranger(row)
	if err != nil {
		return nil, fmt.Errorf("StrangerRepository.GetBySessionID: %w", err)
	}
	return s, nil
}

// GetByID fetches a stranger record by its UUID within an org.
func (r *StrangerRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, id string) (*model.StrangerUser, error) {
	row := tx.QueryRow(ctx, sqlGetStrangerByID, orgID, id)
	s, err := scanStranger(row)
	if err != nil {
		return nil, fmt.Errorf("StrangerRepository.GetByID: %w", err)
	}
	return s, nil
}

// List returns stranger records for an org, optionally filtered by status, with pagination.
// Returns the records and the total count before pagination.
func (r *StrangerRepository) List(ctx context.Context, tx pgx.Tx, orgID string, status *model.StrangerStatus, limit, offset int) ([]model.StrangerUser, int, error) {
	var total int
	if status != nil {
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM stranger_users WHERE org_id = $1 AND status = $2`,
			orgID, *status,
		).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("StrangerRepository.List count: %w", err)
		}
	} else {
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM stranger_users WHERE org_id = $1`,
			orgID,
		).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("StrangerRepository.List count: %w", err)
		}
	}

	var rows pgx.Rows
	var err error
	if status != nil {
		rows, err = tx.Query(ctx, sqlListStrangersByStatus, orgID, *status, limit, offset)
	} else {
		rows, err = tx.Query(ctx, sqlListStrangers, orgID, limit, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("StrangerRepository.List query: %w", err)
	}
	defer rows.Close()

	var strangers []model.StrangerUser
	for rows.Next() {
		var s model.StrangerUser
		var ipStr string
		var blockedBy string
		if err := rows.Scan(
			&s.ID, &s.OrgID, &s.SessionID,
			&ipStr,
			&s.UserAgent,
			&s.Status,
			&s.BlockReason,
			&s.MessageCount, &s.RateLimitRPM,
			&s.LastActiveAt, &s.BlockedAt,
			&blockedBy,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("StrangerRepository.List scan: %w", err)
		}
		if ipStr != "" {
			s.IPAddress = &ipStr
		}
		if blockedBy != "" {
			s.BlockedBy = blockedBy
		}
		strangers = append(strangers, s)
	}
	return strangers, total, rows.Err()
}

// Block updates the status, block_reason, blocked_at, and blocked_by fields.
func (r *StrangerRepository) Block(ctx context.Context, tx pgx.Tx, orgID, id, blockedBy string, req model.BlockStrangerRequest) error {
	tag, err := tx.Exec(ctx,
		`UPDATE stranger_users SET
			status = $3,
			block_reason = $4,
			blocked_at = NOW(),
			blocked_by = $5::uuid
		WHERE org_id = $1 AND id = $2`,
		orgID, id, req.Status, req.Reason, blockedBy,
	)
	if err != nil {
		return fmt.Errorf("StrangerRepository.Block: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("StrangerRepository.Block: stranger %s not found", id)
	}
	return nil
}

// Unblock resets the stranger's status to 'active' and clears block fields.
func (r *StrangerRepository) Unblock(ctx context.Context, tx pgx.Tx, orgID, id string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE stranger_users SET
			status = 'active',
			block_reason = NULL,
			blocked_at = NULL,
			blocked_by = NULL
		WHERE org_id = $1 AND id = $2`,
		orgID, id,
	)
	if err != nil {
		return fmt.Errorf("StrangerRepository.Unblock: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("StrangerRepository.Unblock: stranger %s not found", id)
	}
	return nil
}

// SetRateLimit updates the per-session rate limit (RPM). Passing nil clears the override.
func (r *StrangerRepository) SetRateLimit(ctx context.Context, tx pgx.Tx, orgID, id string, rpm *int) error {
	tag, err := tx.Exec(ctx,
		`UPDATE stranger_users SET rate_limit_rpm = $3 WHERE org_id = $1 AND id = $2`,
		orgID, id, rpm,
	)
	if err != nil {
		return fmt.Errorf("StrangerRepository.SetRateLimit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("StrangerRepository.SetRateLimit: stranger %s not found", id)
	}
	return nil
}

// Delete removes a stranger record by ID.
func (r *StrangerRepository) Delete(ctx context.Context, tx pgx.Tx, orgID, id string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM stranger_users WHERE org_id = $1 AND id = $2`,
		orgID, id,
	)
	if err != nil {
		return fmt.Errorf("StrangerRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("StrangerRepository.Delete: stranger %s not found", id)
	}
	return nil
}

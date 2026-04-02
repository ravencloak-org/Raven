package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
)

// LeadRepository handles database operations for lead profiles.
// All operations set app.current_org_id for RLS enforcement via db.WithOrgID.
type LeadRepository struct {
	pool *pgxpool.Pool
}

// NewLeadRepository creates a new LeadRepository.
func NewLeadRepository(pool *pgxpool.Pool) *LeadRepository {
	return &LeadRepository{pool: pool}
}

const leadColumns = `id, org_id,
	COALESCE(knowledge_base_id::text, '') AS knowledge_base_id,
	COALESCE(session_ids, '{}') AS session_ids,
	COALESCE(email, '') AS email,
	COALESCE(name, '') AS name,
	COALESCE(phone, '') AS phone,
	COALESCE(company, '') AS company,
	COALESCE(metadata, '{}') AS metadata,
	engagement_score, total_messages, total_sessions,
	first_seen_at, last_seen_at, created_at, updated_at`

func scanLead(row pgx.Row) (*model.LeadProfile, error) {
	var l model.LeadProfile
	err := row.Scan(
		&l.ID,
		&l.OrgID,
		&l.KnowledgeBaseID,
		&l.SessionIDs,
		&l.Email,
		&l.Name,
		&l.Phone,
		&l.Company,
		&l.Metadata,
		&l.EngagementScore,
		&l.TotalMessages,
		&l.TotalSessions,
		&l.FirstSeenAt,
		&l.LastSeenAt,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// Upsert inserts a new lead profile or updates an existing one matched by (org_id, email).
func (r *LeadRepository) Upsert(ctx context.Context, orgID string, req model.UpsertLeadRequest) (*model.LeadProfile, error) {
	totalMessages := 0
	if req.TotalMessages != nil {
		totalMessages = *req.TotalMessages
	}
	totalSessions := 1
	if req.TotalSessions != nil {
		totalSessions = *req.TotalSessions
	}

	var lead *model.LeadProfile
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`INSERT INTO lead_profiles
				(org_id, knowledge_base_id, session_ids, email, name, phone, company, metadata, total_messages, total_sessions)
			 VALUES
				($1, $2::uuid, $3::uuid[], NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), COALESCE($8::jsonb, '{}'), $9, $10)
			 ON CONFLICT (org_id, email) DO UPDATE SET
				knowledge_base_id = COALESCE(EXCLUDED.knowledge_base_id, lead_profiles.knowledge_base_id),
				session_ids       = CASE
					WHEN EXCLUDED.session_ids IS NOT NULL AND array_length(EXCLUDED.session_ids, 1) > 0
					THEN (SELECT ARRAY(SELECT DISTINCT unnest(lead_profiles.session_ids || EXCLUDED.session_ids)))
					ELSE lead_profiles.session_ids
				END,
				name              = COALESCE(EXCLUDED.name, lead_profiles.name),
				phone             = COALESCE(EXCLUDED.phone, lead_profiles.phone),
				company           = COALESCE(EXCLUDED.company, lead_profiles.company),
				metadata          = lead_profiles.metadata || EXCLUDED.metadata,
				total_messages    = lead_profiles.total_messages + EXCLUDED.total_messages,
				total_sessions    = lead_profiles.total_sessions + EXCLUDED.total_sessions,
				last_seen_at      = NOW(),
				updated_at        = NOW()
			 RETURNING `+leadColumns,
			orgID, req.KnowledgeBaseID, req.SessionIDs,
			req.Email, req.Name, req.Phone, req.Company,
			req.Metadata, totalMessages, totalSessions,
		)
		var err error
		lead, err = scanLead(row)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("LeadRepository.Upsert: %w", err)
	}
	return lead, nil
}

// GetByID fetches a lead profile by its primary key within an org.
func (r *LeadRepository) GetByID(ctx context.Context, orgID, id string) (*model.LeadProfile, error) {
	var lead *model.LeadProfile
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT `+leadColumns+`
			 FROM lead_profiles
			 WHERE id = $1 AND org_id = $2`,
			id, orgID,
		)
		var err error
		lead, err = scanLead(row)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("LeadRepository.GetByID: %w", err)
	}
	return lead, nil
}

// List returns a paginated list of lead profiles for an org, optionally filtered by minScore.
func (r *LeadRepository) List(ctx context.Context, orgID string, minScore *float32, limit, offset int) ([]model.LeadProfile, int, error) {
	var leads []model.LeadProfile
	var total int

	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		// Count query.
		if minScore != nil {
			if err := tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM lead_profiles WHERE org_id = $1 AND engagement_score >= $2`,
				orgID, *minScore,
			).Scan(&total); err != nil {
				return fmt.Errorf("count: %w", err)
			}
		} else {
			if err := tx.QueryRow(ctx,
				`SELECT COUNT(*) FROM lead_profiles WHERE org_id = $1`,
				orgID,
			).Scan(&total); err != nil {
				return fmt.Errorf("count: %w", err)
			}
		}

		// Data query.
		var rows pgx.Rows
		var err error
		if minScore != nil {
			rows, err = tx.Query(ctx,
				`SELECT `+leadColumns+`
				 FROM lead_profiles
				 WHERE org_id = $1 AND engagement_score >= $2
				 ORDER BY engagement_score DESC, last_seen_at DESC
				 LIMIT $3 OFFSET $4`,
				orgID, *minScore, limit, offset,
			)
		} else {
			rows, err = tx.Query(ctx,
				`SELECT `+leadColumns+`
				 FROM lead_profiles
				 WHERE org_id = $1
				 ORDER BY engagement_score DESC, last_seen_at DESC
				 LIMIT $2 OFFSET $3`,
				orgID, limit, offset,
			)
		}
		if err != nil {
			return fmt.Errorf("query: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var l model.LeadProfile
			if err := rows.Scan(
				&l.ID, &l.OrgID, &l.KnowledgeBaseID, &l.SessionIDs,
				&l.Email, &l.Name, &l.Phone, &l.Company,
				&l.Metadata, &l.EngagementScore, &l.TotalMessages,
				&l.TotalSessions, &l.FirstSeenAt, &l.LastSeenAt,
				&l.CreatedAt, &l.UpdatedAt,
			); err != nil {
				return fmt.Errorf("scan: %w", err)
			}
			leads = append(leads, l)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, fmt.Errorf("LeadRepository.List: %w", err)
	}
	return leads, total, nil
}

// Update applies partial updates to a lead profile.
func (r *LeadRepository) Update(ctx context.Context, orgID, id string, req model.UpdateLeadRequest) (*model.LeadProfile, error) {
	var lead *model.LeadProfile
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`UPDATE lead_profiles
			 SET
			   knowledge_base_id = COALESCE($3::uuid, knowledge_base_id),
			   session_ids       = CASE WHEN $4::uuid[] IS NOT NULL THEN $4::uuid[] ELSE session_ids END,
			   email             = COALESCE($5, email),
			   name              = COALESCE($6, name),
			   phone             = COALESCE($7, phone),
			   company           = COALESCE($8, company),
			   metadata          = CASE WHEN $9::jsonb IS NOT NULL THEN $9::jsonb ELSE metadata END,
			   total_messages    = COALESCE($10, total_messages),
			   total_sessions    = COALESCE($11, total_sessions),
			   last_seen_at      = NOW(),
			   updated_at        = NOW()
			 WHERE id = $1 AND org_id = $2
			 RETURNING `+leadColumns,
			id, orgID,
			req.KnowledgeBaseID, req.SessionIDs,
			req.Email, req.Name, req.Phone, req.Company,
			req.Metadata, req.TotalMessages, req.TotalSessions,
		)
		var err error
		lead, err = scanLead(row)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("LeadRepository.Update: %w", err)
	}
	return lead, nil
}

// Delete permanently removes a lead profile.
func (r *LeadRepository) Delete(ctx context.Context, orgID, id string) error {
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM lead_profiles WHERE id = $1 AND org_id = $2`,
			id, orgID,
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("lead %s not found", id)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("LeadRepository.Delete: %w", err)
	}
	return nil
}

// ExportCSV returns all lead profiles for an org (for CSV export).
func (r *LeadRepository) ExportCSV(ctx context.Context, orgID string) ([]model.LeadProfile, error) {
	var leads []model.LeadProfile
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT `+leadColumns+`
			 FROM lead_profiles
			 WHERE org_id = $1
			 ORDER BY engagement_score DESC, last_seen_at DESC`,
			orgID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l model.LeadProfile
			if err := rows.Scan(
				&l.ID, &l.OrgID, &l.KnowledgeBaseID, &l.SessionIDs,
				&l.Email, &l.Name, &l.Phone, &l.Company,
				&l.Metadata, &l.EngagementScore, &l.TotalMessages,
				&l.TotalSessions, &l.FirstSeenAt, &l.LastSeenAt,
				&l.CreatedAt, &l.UpdatedAt,
			); err != nil {
				return err
			}
			leads = append(leads, l)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("LeadRepository.ExportCSV: %w", err)
	}
	return leads, nil
}

package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
)

// IdentityRepository handles database operations for cross-channel user identities.
type IdentityRepository struct {
	pool *pgxpool.Pool
}

// NewIdentityRepository creates a new IdentityRepository.
func NewIdentityRepository(pool *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{pool: pool}
}

const identityCols = `id, org_id, anonymous_id,
	user_id::text, posthog_distinct_id,
	channel, first_seen_at, last_seen_at, session_count,
	COALESCE(metadata, '{}') AS metadata, created_at`

func scanIdentity(row pgx.Row) (*model.UserIdentity, error) {
	var identity model.UserIdentity
	var userID *string
	var metadataBytes []byte

	err := row.Scan(
		&identity.ID, &identity.OrgID, &identity.AnonymousID,
		&userID, &identity.PosthogDistinctID,
		&identity.Channel, &identity.FirstSeenAt, &identity.LastSeenAt,
		&identity.SessionCount, &metadataBytes, &identity.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	identity.UserID = userID
	if len(metadataBytes) > 2 {
		_ = json.Unmarshal(metadataBytes, &identity.Metadata)
	}
	if identity.Metadata == nil {
		identity.Metadata = map[string]any{}
	}
	return &identity, nil
}

// Upsert inserts or updates a user identity record within an RLS-guarded transaction.
// On conflict (org_id, anonymous_id, channel) it increments session_count, updates
// last_seen_at, and merges user_id / posthog_distinct_id if provided.
func (r *IdentityRepository) Upsert(ctx context.Context, orgID string, req model.IdentifyRequest) (*model.UserIdentity, error) {
	metadataBytes, err := json.Marshal(req.Metadata)
	if err != nil {
		metadataBytes = []byte("{}")
	}
	if req.Metadata == nil {
		metadataBytes = []byte("{}")
	}

	var userIDArg any
	if req.UserID != "" {
		userIDArg = req.UserID
	}

	var posthogArg any
	if req.PosthogDistinctID != "" {
		posthogArg = req.PosthogDistinctID
	}

	var identity *model.UserIdentity
	err = db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`INSERT INTO user_identities
				(org_id, anonymous_id, user_id, posthog_distinct_id, channel, metadata)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (org_id, anonymous_id, channel) DO UPDATE SET
				last_seen_at           = NOW(),
				session_count          = user_identities.session_count + 1,
				user_id                = COALESCE(EXCLUDED.user_id, user_identities.user_id),
				posthog_distinct_id    = COALESCE(EXCLUDED.posthog_distinct_id, user_identities.posthog_distinct_id)
			RETURNING `+identityCols,
			orgID, req.AnonymousID, userIDArg, posthogArg, req.Channel, metadataBytes,
		)
		var e error
		identity, e = scanIdentity(row)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("IdentityRepository.Upsert: %w", err)
	}
	return identity, nil
}

// GetByAnonymousID fetches a user identity by anonymous_id and channel within an org.
func (r *IdentityRepository) GetByAnonymousID(ctx context.Context, orgID, anonymousID, channel string) (*model.UserIdentity, error) {
	var identity *model.UserIdentity
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT `+identityCols+`
			FROM user_identities
			WHERE org_id = $1 AND anonymous_id = $2 AND channel = $3`,
			orgID, anonymousID, channel,
		)
		var e error
		identity, e = scanIdentity(row)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("IdentityRepository.GetByAnonymousID: %w", err)
	}
	return identity, nil
}

// List returns a paginated list of user identities for an org, ordered by last_seen_at DESC.
func (r *IdentityRepository) List(ctx context.Context, orgID string, limit, offset int) ([]model.UserIdentity, int, error) {
	var identities []model.UserIdentity
	var total int

	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		if e := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM user_identities WHERE org_id = $1`, orgID,
		).Scan(&total); e != nil {
			return fmt.Errorf("count: %w", e)
		}

		rows, e := tx.Query(ctx,
			`SELECT `+identityCols+`
			FROM user_identities
			WHERE org_id = $1
			ORDER BY last_seen_at DESC
			LIMIT $2 OFFSET $3`,
			orgID, limit, offset,
		)
		if e != nil {
			return fmt.Errorf("query: %w", e)
		}
		defer rows.Close()

		for rows.Next() {
			var identity model.UserIdentity
			var userID *string
			var metadataBytes []byte
			if e2 := rows.Scan(
				&identity.ID, &identity.OrgID, &identity.AnonymousID,
				&userID, &identity.PosthogDistinctID,
				&identity.Channel, &identity.FirstSeenAt, &identity.LastSeenAt,
				&identity.SessionCount, &metadataBytes, &identity.CreatedAt,
			); e2 != nil {
				return fmt.Errorf("scan: %w", e2)
			}
			identity.UserID = userID
			if len(metadataBytes) > 2 {
				_ = json.Unmarshal(metadataBytes, &identity.Metadata)
			}
			if identity.Metadata == nil {
				identity.Metadata = map[string]any{}
			}
			identities = append(identities, identity)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, fmt.Errorf("IdentityRepository.List: %w", err)
	}
	if identities == nil {
		identities = []model.UserIdentity{}
	}
	return identities, total, nil
}

// Delete removes a user identity record by ID within an org.
func (r *IdentityRepository) Delete(ctx context.Context, orgID, id string) error {
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		tag, e := tx.Exec(ctx,
			`DELETE FROM user_identities WHERE id = $1 AND org_id = $2`,
			id, orgID,
		)
		if e != nil {
			return e
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("identity %s not found", id)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("IdentityRepository.Delete: %w", err)
	}
	return nil
}

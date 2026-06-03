package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/model"
)

// OrgRepository handles database operations for organisations.
// It does NOT enforce RLS itself — callers must use db.WithOrgID when operating
// inside a tenant context. Org creation and admin reads run without RLS.
type OrgRepository struct {
	pool *pgxpool.Pool
}

// NewOrgRepository creates a new OrgRepository backed by pool.
func NewOrgRepository(pool *pgxpool.Pool) *OrgRepository {
	return &OrgRepository{pool: pool}
}

func scanOrg(row pgx.Row) (*model.Organization, error) {
	var org model.Organization
	err := row.Scan(
		&org.ID,
		&org.Name,
		&org.Slug,
		&org.Status,
		&org.Settings,
		&org.CreatedAt,
		&org.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// Create inserts a new organisation and returns the persisted record.
func (r *OrgRepository) Create(ctx context.Context, name, slug string) (*model.Organization, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug)
		 VALUES ($1, $2)
		 RETURNING id, name, slug, status, settings, created_at, updated_at`,
		name, slug,
	)
	org, err := scanOrg(row)
	if err != nil {
		return nil, fmt.Errorf("OrgRepository.Create: %w", err)
	}
	return org, nil
}

// GetByID fetches a non-deactivated organisation by its UUID.
func (r *OrgRepository) GetByID(ctx context.Context, orgID string) (*model.Organization, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, status, settings, created_at, updated_at
		 FROM organizations
		 WHERE id = $1 AND status != 'deactivated'`,
		orgID,
	)
	org, err := scanOrg(row)
	if err != nil {
		return nil, fmt.Errorf("OrgRepository.GetByID: %w", err)
	}
	return org, nil
}

// Update applies partial updates (name and/or settings) to an organisation.
func (r *OrgRepository) Update(ctx context.Context, orgID string, name *string, settings map[string]any) (*model.Organization, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE organizations
		 SET
		   name     = COALESCE($2, name),
		   settings = CASE WHEN $3::jsonb IS NOT NULL THEN $3::jsonb ELSE settings END
		 WHERE id = $1 AND status != 'deactivated'
		 RETURNING id, name, slug, status, settings, created_at, updated_at`,
		orgID, name, settings,
	)
	org, err := scanOrg(row)
	if err != nil {
		return nil, fmt.Errorf("OrgRepository.Update: %w", err)
	}
	return org, nil
}

// GetBySlug fetches a non-deactivated organisation by its slug.
// Returns (nil, nil) when no matching organisation exists.
func (r *OrgRepository) GetBySlug(ctx context.Context, slug string) (*model.Organization, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, status, settings, created_at, updated_at
		 FROM organizations
		 WHERE slug = $1 AND status != 'deactivated'`,
		slug,
	)
	org, err := scanOrg(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("OrgRepository.GetBySlug: %w", err)
	}
	return org, nil
}

// RenameSlug atomically swaps an organisation's slug and registers the
// previous value as a 30-day soft-redirect in `org_slug_holds`. The whole
// operation runs in a single transaction (see marketplace.RenameOrgSlug):
// a crash between the swap and the hold-insert can never produce a dead
// redirect. Returns marketplace.ErrInvalidSlug, marketplace.ErrSlugTaken,
// or marketplace.ErrSlugNotFound on the documented failure modes.
func (r *OrgRepository) RenameSlug(ctx context.Context, orgID, newSlug string) error {
	return marketplace.RenameOrgSlug(ctx, r.pool, orgID, newSlug)
}

// ResolveSlug maps a public Marketplace URL slug to its current org, going
// via the `org_slug_holds` soft-redirect table on a live-slug miss. The
// returned ResolvedOrg.IsRedirect tells the URL handler whether to serve
// a 200 (live) or a 301 to ResolvedOrg.CanonicalSlug (redirect). Returns
// marketplace.ErrSlugNotFound on a full miss.
func (r *OrgRepository) ResolveSlug(ctx context.Context, slug string) (marketplace.ResolvedOrg, error) {
	return marketplace.ResolveOrgSlug(ctx, r.pool, slug)
}

// SoftDelete sets the organisation status to 'deactivated'.
func (r *OrgRepository) SoftDelete(ctx context.Context, orgID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE organizations SET status = 'deactivated' WHERE id = $1`,
		orgID,
	)
	if err != nil {
		return fmt.Errorf("OrgRepository.SoftDelete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("OrgRepository.SoftDelete: organisation %s not found", orgID)
	}
	return nil
}

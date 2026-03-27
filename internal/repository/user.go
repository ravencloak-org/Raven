package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// UserRepository handles database operations for users.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const userColumns = `id, org_id, email, COALESCE(display_name, '') AS display_name,
	COALESCE(keycloak_sub, '') AS keycloak_sub, status, last_login_at, created_at, updated_at`

func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	err := row.Scan(
		&u.ID,
		&u.OrgID,
		&u.Email,
		&u.DisplayName,
		&u.KeycloakSub,
		&u.Status,
		&u.LastLoginAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpsertByKeycloakSub creates or updates a user based on Keycloak subject claim.
// This is the primary sync path called from the Keycloak webhook handler.
func (r *UserRepository) UpsertByKeycloakSub(ctx context.Context, orgID, keycloakSub, email, displayName string) (*model.User, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO users (org_id, email, display_name, keycloak_sub)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (keycloak_sub) DO UPDATE
		   SET email        = EXCLUDED.email,
		       display_name = COALESCE(EXCLUDED.display_name, users.display_name),
		       updated_at   = NOW()
		 RETURNING `+userColumns,
		orgID, email, displayName, keycloakSub,
	)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("UserRepository.UpsertByKeycloakSub: %w", err)
	}
	return u, nil
}

// GetByID fetches an active user by primary key.
func (r *UserRepository) GetByID(ctx context.Context, userID string) (*model.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1 AND status = 'active'`,
		userID,
	)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("UserRepository.GetByID: %w", err)
	}
	return u, nil
}

// GetByKeycloakSub fetches a user by their Keycloak subject identifier.
func (r *UserRepository) GetByKeycloakSub(ctx context.Context, sub string) (*model.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE keycloak_sub = $1 AND status = 'active'`,
		sub,
	)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("UserRepository.GetByKeycloakSub: %w", err)
	}
	return u, nil
}

// UpdateDisplayName changes a user's display name.
func (r *UserRepository) UpdateDisplayName(ctx context.Context, userID string, displayName *string) (*model.User, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE users
		 SET display_name = COALESCE($2, display_name)
		 WHERE id = $1 AND status = 'active'
		 RETURNING `+userColumns,
		userID, displayName,
	)
	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("UserRepository.UpdateDisplayName: %w", err)
	}
	return u, nil
}

// SoftDelete sets user status to 'disabled' (GDPR-safe — no data purge).
func (r *UserRepository) SoftDelete(ctx context.Context, userID string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE users SET status = 'disabled' WHERE id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("UserRepository.SoftDelete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("UserRepository.SoftDelete: user %s not found", userID)
	}
	return nil
}

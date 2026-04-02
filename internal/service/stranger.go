package service

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// StrangerService provides business logic for anonymous stranger user management.
type StrangerService struct {
	repo *repository.StrangerRepository
	pool *pgxpool.Pool
}

// NewStrangerService creates a new StrangerService.
func NewStrangerService(repo *repository.StrangerRepository, pool *pgxpool.Pool) *StrangerService {
	return &StrangerService{repo: repo, pool: pool}
}

// Upsert tracks an anonymous session, creating or updating the stranger record.
func (s *StrangerService) Upsert(ctx context.Context, orgID string, req model.UpsertStrangerRequest) (*model.StrangerUser, error) {
	var stranger *model.StrangerUser
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		stranger, e = s.repo.Upsert(ctx, tx, orgID, req)
		return e
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to upsert stranger: " + err.Error())
	}
	return stranger, nil
}

// GetBySessionID returns a stranger record by session ID.
func (s *StrangerService) GetBySessionID(ctx context.Context, orgID, sessionID string) (*model.StrangerUser, error) {
	var stranger *model.StrangerUser
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		stranger, e = s.repo.GetBySessionID(ctx, tx, orgID, sessionID)
		return e
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("stranger not found")
		}
		return nil, apierror.NewInternal("failed to get stranger: " + err.Error())
	}
	return stranger, nil
}

// GetByID returns a stranger record by its UUID.
func (s *StrangerService) GetByID(ctx context.Context, orgID, id string) (*model.StrangerUser, error) {
	var stranger *model.StrangerUser
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		stranger, e = s.repo.GetByID(ctx, tx, orgID, id)
		return e
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("stranger not found")
		}
		return nil, apierror.NewInternal("failed to get stranger: " + err.Error())
	}
	return stranger, nil
}

// List returns stranger records for an org with optional status filter and pagination.
func (s *StrangerService) List(ctx context.Context, orgID string, status *model.StrangerStatus, limit, offset int) ([]model.StrangerUser, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var strangers []model.StrangerUser
	var total int
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		strangers, total, e = s.repo.List(ctx, tx, orgID, status, limit, offset)
		return e
	})
	if err != nil {
		return nil, 0, apierror.NewInternal("failed to list strangers: " + err.Error())
	}
	if strangers == nil {
		strangers = []model.StrangerUser{}
	}
	return strangers, total, nil
}

// Block sets the status of a stranger to blocked or banned with a reason.
func (s *StrangerService) Block(ctx context.Context, orgID, id, blockedBy string, req model.BlockStrangerRequest) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Block(ctx, tx, orgID, id, blockedBy, req)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("stranger not found")
		}
		return apierror.NewInternal("failed to block stranger: " + err.Error())
	}
	return nil
}

// Unblock resets a stranger's status to active.
func (s *StrangerService) Unblock(ctx context.Context, orgID, id string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Unblock(ctx, tx, orgID, id)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("stranger not found")
		}
		return apierror.NewInternal("failed to unblock stranger: " + err.Error())
	}
	return nil
}

// SetRateLimit overrides the per-session rate limit (RPM) for a stranger.
func (s *StrangerService) SetRateLimit(ctx context.Context, orgID, id string, rpm *int) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.SetRateLimit(ctx, tx, orgID, id, rpm)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("stranger not found")
		}
		return apierror.NewInternal("failed to set rate limit: " + err.Error())
	}
	return nil
}

// Delete removes a stranger record from the database.
func (s *StrangerService) Delete(ctx context.Context, orgID, id string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, orgID, id)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("stranger not found")
		}
		return apierror.NewInternal("failed to delete stranger: " + err.Error())
	}
	return nil
}

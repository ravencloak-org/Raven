package service

import (
	"context"
	"log/slog"
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
		slog.WarnContext(ctx, "StrangerService.Upsert db error", "error", err)
	return nil, apierror.NewInternal("internal error")
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
		slog.WarnContext(ctx, "StrangerService.GetBySessionID db error", "error", err)
		return nil, apierror.NewInternal("internal error")
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
		slog.WarnContext(ctx, "StrangerService.GetByID db error", "error", err)
		return nil, apierror.NewInternal("internal error")
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
		slog.WarnContext(ctx, "StrangerService.List db error", "error", err)
		return nil, 0, apierror.NewInternal("internal error")
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
		slog.WarnContext(ctx, "StrangerService.Block db error", "error", err)
		return apierror.NewInternal("internal error")
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
		slog.WarnContext(ctx, "StrangerService.Unblock db error", "error", err)
		return apierror.NewInternal("internal error")
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
		slog.WarnContext(ctx, "StrangerService.SetRateLimit db error", "error", err)
		return apierror.NewInternal("internal error")
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
		slog.WarnContext(ctx, "StrangerService.Delete db error", "error", err)
		return apierror.NewInternal("internal error")
	}
	return nil
}

// FlagSuspicious promotes an active stranger to "throttled" status when
// suspicious burst behaviour is detected by the middleware. It is a
// best-effort call: if the stranger is already throttled/blocked/banned the
// update is a no-op at the database level (the WHERE clause filters them out).
func (s *StrangerService) FlagSuspicious(ctx context.Context, orgID, id string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		_, execErr := tx.Exec(ctx,
			`UPDATE stranger_users
			 SET status = 'throttled',
			     block_reason = 'auto-throttled: suspicious burst activity detected'
			 WHERE org_id = $1 AND id = $2 AND status = 'active'`,
			orgID, id,
		)
		return execErr
	})
	if err != nil {
		slog.WarnContext(ctx, "StrangerService.FlagSuspicious db error", "error", err)
		return apierror.NewInternal("internal error")
	}
	return nil
}

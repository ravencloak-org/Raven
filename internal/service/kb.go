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

// KBService contains business logic for knowledge base management.
type KBService struct {
	repo  *repository.KBRepository
	pool  *pgxpool.Pool
	quota QuotaCheckerI
}

// NewKBService creates a new KBService.
func NewKBService(repo *repository.KBRepository, pool *pgxpool.Pool, quota QuotaCheckerI) *KBService {
	return &KBService{repo: repo, pool: pool, quota: quota}
}

// Create validates and creates a new knowledge base within a workspace.
func (s *KBService) Create(ctx context.Context, orgID, wsID string, req model.CreateKBRequest) (*model.KnowledgeBase, error) {
	if s.quota != nil {
		if err := s.quota.CheckKBQuota(ctx, orgID); err != nil {
			return nil, err
		}
	}

	slug := toSlug(req.Name)
	if slug == "" {
		return nil, apierror.NewBadRequest("knowledge base name produces an empty slug")
	}
	var kb *model.KnowledgeBase
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		kb, err = s.repo.Create(ctx, tx, orgID, wsID, req.Name, slug, req.Description)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apierror.NewBadRequest("knowledge base slug already taken in this workspace: " + slug)
		}
		return nil, apierror.NewInternal("failed to create knowledge base: " + err.Error())
	}
	return kb, nil
}

// GetByID retrieves an active knowledge base by ID within an org.
func (s *KBService) GetByID(ctx context.Context, orgID, kbID string) (*model.KnowledgeBase, error) {
	var kb *model.KnowledgeBase
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		kb, err = s.repo.GetByID(ctx, tx, orgID, kbID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("knowledge base not found")
		}
		return nil, apierror.NewInternal("failed to fetch knowledge base: " + err.Error())
	}
	return kb, nil
}

// ListByWorkspace returns all active KBs in a workspace.
func (s *KBService) ListByWorkspace(ctx context.Context, orgID, wsID string) ([]model.KnowledgeBase, error) {
	var kbs []model.KnowledgeBase
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		kbs, err = s.repo.ListByWorkspace(ctx, tx, orgID, wsID)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list knowledge bases: " + err.Error())
	}
	return kbs, nil
}

// Update applies partial updates to a knowledge base.
func (s *KBService) Update(ctx context.Context, orgID, kbID string, req model.UpdateKBRequest) (*model.KnowledgeBase, error) {
	var kb *model.KnowledgeBase
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		kb, err = s.repo.Update(ctx, tx, orgID, kbID, req.Name, req.Description, req.Settings)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("knowledge base not found")
		}
		return nil, apierror.NewInternal("failed to update knowledge base: " + err.Error())
	}
	return kb, nil
}

// Archive archives (soft-deletes) a knowledge base.
func (s *KBService) Archive(ctx context.Context, orgID, kbID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Archive(ctx, tx, orgID, kbID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("knowledge base not found")
		}
		return apierror.NewInternal("failed to archive knowledge base: " + err.Error())
	}
	return nil
}

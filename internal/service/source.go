package service

import (
	"context"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// SourceService contains business logic for source management.
type SourceService struct {
	repo *repository.SourceRepository
	pool *pgxpool.Pool
}

// NewSourceService creates a new SourceService.
func NewSourceService(repo *repository.SourceRepository, pool *pgxpool.Pool) *SourceService {
	return &SourceService{repo: repo, pool: pool}
}

// validSourceTypes is the set of allowed source_type values.
var validSourceTypes = map[model.SourceType]bool{
	model.SourceTypeWebPage: true,
	model.SourceTypeWebSite: true,
	model.SourceTypeSitemap: true,
	model.SourceTypeRSSFeed: true,
}

// validCrawlFrequencies is the set of allowed crawl_frequency values.
var validCrawlFrequencies = map[model.CrawlFrequency]bool{
	model.CrawlFrequencyManual:  true,
	model.CrawlFrequencyDaily:   true,
	model.CrawlFrequencyWeekly:  true,
	model.CrawlFrequencyMonthly: true,
}

// validateURL ensures the URL is a valid HTTP or HTTPS URL.
func validateURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return apierror.NewBadRequest("invalid URL: " + err.Error())
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return apierror.NewBadRequest("URL must use http or https scheme")
	}
	if u.Host == "" {
		return apierror.NewBadRequest("URL must have a valid host")
	}
	return nil
}

// validateCrawlDepth ensures the crawl depth is between 1 and 5 inclusive.
func validateCrawlDepth(depth *int) error {
	if depth == nil {
		return nil
	}
	if *depth < 1 || *depth > 5 {
		return apierror.NewBadRequest("crawl_depth must be between 1 and 5")
	}
	return nil
}

// Create validates and persists a new source.
func (s *SourceService) Create(ctx context.Context, orgID, kbID string, req model.CreateSourceRequest, createdBy string) (*model.Source, error) {
	if !validSourceTypes[req.SourceType] {
		return nil, apierror.NewBadRequest("invalid source_type: " + string(req.SourceType))
	}
	if err := validateURL(req.URL); err != nil {
		return nil, err
	}
	if err := validateCrawlDepth(req.CrawlDepth); err != nil {
		return nil, err
	}
	if req.CrawlFrequency != "" && !validCrawlFrequencies[req.CrawlFrequency] {
		return nil, apierror.NewBadRequest("invalid crawl_frequency: " + string(req.CrawlFrequency))
	}

	var src *model.Source
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		src, err = s.repo.Create(ctx, tx, orgID, kbID, req, createdBy)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "foreign key") || strings.Contains(err.Error(), "violates") {
			return nil, apierror.NewBadRequest("knowledge base not found or invalid reference")
		}
		return nil, apierror.NewInternal("failed to create source: " + err.Error())
	}
	return src, nil
}

// GetByID retrieves a source by ID within an org.
func (s *SourceService) GetByID(ctx context.Context, orgID, sourceID string) (*model.Source, error) {
	var src *model.Source
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		src, err = s.repo.GetByID(ctx, tx, orgID, sourceID)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("source not found")
		}
		return nil, apierror.NewInternal("failed to fetch source: " + err.Error())
	}
	return src, nil
}

// List returns a paginated list of sources for a knowledge base.
func (s *SourceService) List(ctx context.Context, orgID, kbID string, page, pageSize int) (*model.SourceListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var sources []model.Source
	var total int
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		sources, total, err = s.repo.List(ctx, tx, orgID, kbID, page, pageSize)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list sources: " + err.Error())
	}
	if sources == nil {
		sources = []model.Source{}
	}
	return &model.SourceListResponse{
		Data:     sources,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Update validates and applies partial updates to a source.
func (s *SourceService) Update(ctx context.Context, orgID, sourceID string, req model.UpdateSourceRequest) (*model.Source, error) {
	if req.URL != nil {
		if err := validateURL(*req.URL); err != nil {
			return nil, err
		}
	}
	if err := validateCrawlDepth(req.CrawlDepth); err != nil {
		return nil, err
	}
	if req.CrawlFrequency != nil && !validCrawlFrequencies[*req.CrawlFrequency] {
		return nil, apierror.NewBadRequest("invalid crawl_frequency: " + string(*req.CrawlFrequency))
	}

	var src *model.Source
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		src, err = s.repo.Update(ctx, tx, orgID, sourceID, req)
		return err
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("source not found")
		}
		return nil, apierror.NewInternal("failed to update source: " + err.Error())
	}
	return src, nil
}

// Delete permanently removes a source.
func (s *SourceService) Delete(ctx context.Context, orgID, sourceID string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, orgID, sourceID)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("source not found")
		}
		return apierror.NewInternal("failed to delete source: " + err.Error())
	}
	return nil
}

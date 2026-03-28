package service

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

const (
	defaultSearchLimit = 10
	maxSearchLimit     = 100
)

// SearchService contains business logic for full-text search operations.
type SearchService struct {
	repo *repository.SearchRepository
	pool *pgxpool.Pool
}

// NewSearchService creates a new SearchService.
func NewSearchService(repo *repository.SearchRepository, pool *pgxpool.Pool) *SearchService {
	return &SearchService{repo: repo, pool: pool}
}

// sanitizeQuery trims whitespace and collapses multiple spaces.
func sanitizeQuery(q string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(q)), " ")
}

// clampLimit ensures limit is within [1, maxSearchLimit].
func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultSearchLimit
	}
	if limit > maxSearchLimit {
		return maxSearchLimit
	}
	return limit
}

// TextSearch performs a full-text search across chunks in a knowledge base.
func (s *SearchService) TextSearch(ctx context.Context, orgID, kbID, query string, limit int) (*model.SearchResponse, error) {
	q := sanitizeQuery(query)
	if q == "" {
		return &model.SearchResponse{Results: []model.ChunkWithRank{}, Total: 0}, nil
	}
	limit = clampLimit(limit)

	var results []model.ChunkWithRank
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		results, err = s.repo.TextSearch(ctx, tx, orgID, kbID, q, limit)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to search chunks: " + err.Error())
	}
	results = lo.Ternary(results == nil, []model.ChunkWithRank{}, results)
	return &model.SearchResponse{Results: results, Total: len(results)}, nil
}

// TextSearchWithFilters performs a full-text search restricted to specific documents.
func (s *SearchService) TextSearchWithFilters(ctx context.Context, orgID, kbID, query string, docIDs []string, limit int) (*model.SearchResponse, error) {
	q := sanitizeQuery(query)
	if q == "" {
		return &model.SearchResponse{Results: []model.ChunkWithRank{}, Total: 0}, nil
	}
	limit = clampLimit(limit)

	if len(docIDs) == 0 {
		return s.TextSearch(ctx, orgID, kbID, q, limit)
	}

	var results []model.ChunkWithRank
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var err error
		results, err = s.repo.TextSearchWithFilters(ctx, tx, orgID, kbID, q, docIDs, limit)
		return err
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to search chunks: " + err.Error())
	}
	results = lo.Ternary(results == nil, []model.ChunkWithRank{}, results)
	return &model.SearchResponse{Results: results, Total: len(results)}, nil
}

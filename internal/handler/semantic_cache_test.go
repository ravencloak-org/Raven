package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockSemCacheRepo records the last call made to it and returns scripted
// results. It intentionally does NOT perform any SQL; full behaviour is
// covered by the integration suite (internal/integration/cache_test.go).
type mockSemCacheRepo struct {
	invalidateFn func(ctx context.Context, orgID, kbID string) (int64, error)
	statsFn      func(ctx context.Context, orgID, kbID string) (repository.CacheStats, error)
}

func (m *mockSemCacheRepo) InvalidateKB(ctx context.Context, orgID, kbID string) (int64, error) {
	return m.invalidateFn(ctx, orgID, kbID)
}
func (m *mockSemCacheRepo) Stats(ctx context.Context, orgID, kbID string) (repository.CacheStats, error) {
	return m.statsFn(ctx, orgID, kbID)
}

func newSemCacheRouter(repo handler.SemanticCacheRepositorier) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewSemanticCacheHandler(repo)
	r.DELETE("/api/v1/orgs/:org_id/kbs/:kb_id/cache", h.InvalidateKBCache)
	r.GET("/api/v1/orgs/:org_id/kbs/:kb_id/cache/stats", h.GetCacheStats)
	return r
}

// Issue #256 — GET /cache/stats must return the spec-mandated fields.
func TestGetCacheStats_ReturnsSpecShape(t *testing.T) {
	expiresSoonest := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	repo := &mockSemCacheRepo{
		statsFn: func(_ context.Context, _, _ string) (repository.CacheStats, error) {
			return repository.CacheStats{
				TotalEntries:         42,
				TotalHits:            100,
				EstimatedTokensSaved: 100 * 1250,
				ExpiresSoonest:       &expiresSoonest,
				AvgHits:              2.38,
			}, nil
		},
	}
	r := newSemCacheRouter(repo)
	org := uuid.NewString()
	kb := uuid.NewString()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/"+org+"/kbs/"+kb+"/cache/stats", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(t, 42, body["total_entries"])
	assert.EqualValues(t, 100, body["hit_count"])
	assert.EqualValues(t, 125000, body["estimated_tokens_saved"])
	assert.NotEmpty(t, body["expires_soonest"], "spec requires expires_soonest field")
}

func TestGetCacheStats_RejectsInvalidOrgID(t *testing.T) {
	repo := &mockSemCacheRepo{}
	r := newSemCacheRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/not-a-uuid/kbs/"+uuid.NewString()+"/cache/stats", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestInvalidateKBCache_ReturnsDeletedCount(t *testing.T) {
	repo := &mockSemCacheRepo{
		invalidateFn: func(_ context.Context, _, _ string) (int64, error) {
			return 17, nil
		},
	}
	r := newSemCacheRouter(repo)
	org := uuid.NewString()
	kb := uuid.NewString()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/"+org+"/kbs/"+kb+"/cache", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.EqualValues(t, 17, body["deleted"])
}

func TestInvalidateKBCache_RejectsInvalidKBID(t *testing.T) {
	repo := &mockSemCacheRepo{}
	r := newSemCacheRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/"+uuid.NewString()+"/kbs/not-uuid/cache", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

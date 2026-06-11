package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockReImporterService implements handler.ReImporterService for handler
// tests. We use a function field rather than a struct of canned responses
// so each test case can express its expectation in one line.
type mockReImporterService struct {
	fn func(ctx context.Context, orgID, kbID, actorUserID string) (marketplace.ReImportResult, error)
}

func (m *mockReImporterService) ReImport(ctx context.Context, orgID, kbID, actorUserID string) (marketplace.ReImportResult, error) {
	return m.fn(ctx, orgID, kbID, actorUserID)
}

// newReImportRouter builds a minimal Gin engine wired to the Re-import
// handler with a stub auth middleware that pre-populates org_id / user_id —
// mirroring what the real auth middleware does before the handler runs.
//
// orgID="" simulates an unauthenticated session (no auth middleware set it).
func newReImportRouter(svc handler.ReImporterService, orgID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if orgID != "" {
			c.Set(string(middleware.ContextKeyOrgID), orgID)
			c.Set(string(middleware.ContextKeyUserID), "user-1")
		}
		c.Next()
	})
	h := handler.NewMarketplaceReImportHandler(svc)
	r.POST("/api/v1/knowledge_bases/:kb_id/re-import", h.ReImport)
	return r
}

// doPost is a tiny helper that issues the JSON POST and returns the
// recorder so each assertion stays focused on status + body.
func doPost(t *testing.T, r *gin.Engine, kbID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/knowledge_bases/"+kbID+"/re-import", &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestReImportHandler_HappyPath_ReturnsContractShape locks the wire contract
// from issue #730: `{ kb_id, imported_from_revision_at }` on 200. The
// chunks_projected field is additive — clients ignoring it stay correct.
func TestReImportHandler_HappyPath_ReturnsContractShape(t *testing.T) {
	rev := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	svc := &mockReImporterService{
		fn: func(_ context.Context, orgID, kbID, actorUserID string) (marketplace.ReImportResult, error) {
			assert.Equal(t, "org-abc", orgID)
			assert.Equal(t, "kb-1", kbID)
			assert.Equal(t, "user-1", actorUserID)
			return marketplace.ReImportResult{
				KBID:                   kbID,
				ImportedFromRevisionAt: rev,
				ChunksProjected:        7,
			}, nil
		},
	}
	r := newReImportRouter(svc, "org-abc")
	w := doPost(t, r, "kb-1", map[string]any{"confirm": true})

	require.Equal(t, http.StatusOK, w.Code)
	var resp handler.ReImportResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "kb-1", resp.KBID)
	assert.True(t, resp.ImportedFromRevisionAt.Equal(rev))
	assert.Equal(t, 7, resp.ChunksProjected)
}

// TestReImportHandler_MissingConfirm_400 covers the explicit-confirm guard.
// The service must not be called when confirm is absent — Re-import is
// destructive, and the handler is the boundary where we enforce that.
func TestReImportHandler_MissingConfirm_400(t *testing.T) {
	called := false
	svc := &mockReImporterService{fn: func(_ context.Context, _, _, _ string) (marketplace.ReImportResult, error) {
		called = true
		return marketplace.ReImportResult{}, nil
	}}
	r := newReImportRouter(svc, "org-abc")
	w := doPost(t, r, "kb-1", map[string]any{})

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, called, "service must not run when confirm is missing")
	assert.Contains(t, w.Body.String(), "destructive")
}

// TestReImportHandler_FalseConfirm_400 — explicit false is treated the same
// as missing. Belt-and-braces vs. clients that always serialise the field.
func TestReImportHandler_FalseConfirm_400(t *testing.T) {
	svc := &mockReImporterService{fn: func(_ context.Context, _, _, _ string) (marketplace.ReImportResult, error) {
		t.Fatal("service must not run when confirm=false")
		return marketplace.ReImportResult{}, nil
	}}
	r := newReImportRouter(svc, "org-abc")
	w := doPost(t, r, "kb-1", map[string]any{"confirm": false})
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestReImportHandler_MalformedBody_400 — a body that doesn't decode is a
// client error, not a 500. The error middleware turns the bind error into
// an apierror.BadRequest.
func TestReImportHandler_MalformedBody_400(t *testing.T) {
	svc := &mockReImporterService{fn: func(_ context.Context, _, _, _ string) (marketplace.ReImportResult, error) {
		t.Fatal("service must not run on malformed body")
		return marketplace.ReImportResult{}, nil
	}}
	gin.SetMode(gin.TestMode)
	r := newReImportRouter(svc, "org-abc")

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/knowledge_bases/kb-1/re-import", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// TestReImportHandler_MissingOrg_401 — when no auth middleware set org_id,
// the handler must surface 401 (auth-layer error), not 500.
func TestReImportHandler_MissingOrg_401(t *testing.T) {
	svc := &mockReImporterService{fn: func(_ context.Context, _, _, _ string) (marketplace.ReImportResult, error) {
		t.Fatal("service must not run without org binding")
		return marketplace.ReImportResult{}, nil
	}}
	r := newReImportRouter(svc, "")
	w := doPost(t, r, "kb-1", map[string]any{"confirm": true})
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestReImportHandler_ServiceErrors_MapsToCorrectStatus is the table that
// pins the service-error → HTTP-status mapping. Future error additions to
// the service belong here as a new row.
func TestReImportHandler_ServiceErrors_MapsToCorrectStatus(t *testing.T) {
	cases := []struct {
		name    string
		svcErr  error
		want    int
		bodyHas string
	}{
		{"not found → 404", marketplace.ErrKBNotFound, http.StatusNotFound, "not found"},
		{"not an import → 409", marketplace.ErrNotAnImport, http.StatusConflict, "authored locally"},
		{"source unpublished → 410", marketplace.ErrSourceUnpublished, http.StatusGone, "unpublished"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svc := &mockReImporterService{fn: func(_ context.Context, _, _, _ string) (marketplace.ReImportResult, error) {
				return marketplace.ReImportResult{}, tc.svcErr
			}}
			r := newReImportRouter(svc, "org-abc")
			w := doPost(t, r, "kb-1", map[string]any{"confirm": true})
			require.Equal(t, tc.want, w.Code, "body: %s", w.Body.String())
			assert.Contains(t, w.Body.String(), tc.bodyHas)
		})
	}
}

// TestReImportHandler_EmptyKBID_400 covers the URL-shape guard. Gin will
// not route to the handler with an empty path segment, so we simulate the
// boundary directly: an explicit empty param shouldn't pass.
//
// In practice the route :kb_id makes this branch dead code at the gin
// level — the test exists to prevent a refactor from quietly removing the
// guard and trusting whatever the URL parser yields.
func TestReImportHandler_EmptyKBID_400(t *testing.T) {
	svc := &mockReImporterService{fn: func(_ context.Context, _, _, _ string) (marketplace.ReImportResult, error) {
		t.Fatal("service must not run with empty kb_id")
		return marketplace.ReImportResult{}, nil
	}}
	h := handler.NewMarketplaceReImportHandler(svc)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Next()
	})
	// Route without :kb_id so c.Param("kb_id") returns "".
	r.POST("/x/re-import", h.ReImport)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/x/re-import",
		strings.NewReader(`{"confirm":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

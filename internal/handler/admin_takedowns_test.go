package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeAuditLister stubs the small surface of *marketplace.Takedowns that
// the handler depends on. Recording the args lets tests assert that
// limit + cursor were forwarded verbatim.
type fakeAuditLister struct {
	rows        []marketplace.AuditRow
	nextCursor  string
	err         error
	gotLimit    int
	gotCursor   string
	called      int
}

func (f *fakeAuditLister) ListAudit(_ context.Context, limit int, cursor string) ([]marketplace.AuditRow, string, error) {
	f.called++
	f.gotLimit = limit
	f.gotCursor = cursor
	return f.rows, f.nextCursor, f.err
}

func mountAdminTakedowns(t *testing.T, repo handler.TakedownsAuditLister, email, allowlist string) *gin.Engine {
	t.Helper()
	t.Setenv("RAVEN_ADMIN_EMAILS", allowlist)
	middleware.ResetAdminEmailsCacheForTest()
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		if email != "" {
			c.Set(string(middleware.ContextKeyEmail), email)
		}
		c.Next()
	})
	h := handler.NewAdminTakedownsHandler(repo)
	r.GET("/api/v1/admin/marketplace/takedowns",
		middleware.RequireRavenAdmin(),
		h.List,
	)
	return r
}

func TestAdminTakedowns_Unauthorized(t *testing.T) {
	r := mountAdminTakedowns(t, &fakeAuditLister{}, "", "admin@raven.org")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/marketplace/takedowns", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestAdminTakedowns_Forbidden(t *testing.T) {
	r := mountAdminTakedowns(t, &fakeAuditLister{}, "user@example.com", "admin@raven.org")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/marketplace/takedowns", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestAdminTakedowns_OK_EmptyPage(t *testing.T) {
	repo := &fakeAuditLister{}
	r := mountAdminTakedowns(t, repo, "admin@raven.org", "admin@raven.org")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/marketplace/takedowns", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Items      []map[string]any `json:"items"`
		NextCursor string           `json:"next_cursor,omitempty"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 0 {
		t.Errorf("items: want 0, got %d", len(body.Items))
	}
	if body.NextCursor != "" {
		t.Errorf("next_cursor: want empty, got %q", body.NextCursor)
	}
}

func TestAdminTakedowns_OK_PopulatedRowAndCursor(t *testing.T) {
	now := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)
	tdID := uuid.New()
	kbID := uuid.New()
	repo := &fakeAuditLister{
		rows: []marketplace.AuditRow{{
			TakedownID:           tdID,
			TargetKBID:           kbID,
			TargetKBName:         "Recipes",
			TargetOrgSlug:        "acme",
			TargetOrgDisplayName: "Acme Corp",
			Source:               marketplace.TakedownSourceAdmin,
			Notes:                "trademark dispute",
			CreatedAt:            now,
			StrikesAfterOrgTotal: 2,
		}},
		nextCursor: "opaque-next",
	}
	r := mountAdminTakedowns(t, repo, "admin@raven.org", "admin@raven.org")
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/marketplace/takedowns?limit=25&cursor=opaque-prev", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if repo.gotLimit != 25 || repo.gotCursor != "opaque-prev" {
		t.Errorf("repo args: limit=%d cursor=%q", repo.gotLimit, repo.gotCursor)
	}
	var body struct {
		Items []struct {
			TakedownID           string `json:"takedown_id"`
			TargetKBID           string `json:"target_kb_id"`
			TargetKBName         string `json:"target_kb_name"`
			TargetOrgSlug        string `json:"target_org_slug"`
			TargetOrgDisplayName string `json:"target_org_display_name"`
			Source               string `json:"source"`
			Notes                string `json:"notes"`
			CreatedAt            string `json:"created_at"`
			Strikes              int64  `json:"strikes_after_org_total"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("items: want 1, got %d", len(body.Items))
	}
	it := body.Items[0]
	if it.TakedownID != tdID.String() || it.TargetKBID != kbID.String() ||
		it.TargetKBName != "Recipes" || it.TargetOrgSlug != "acme" ||
		it.TargetOrgDisplayName != "Acme Corp" || it.Source != "admin" ||
		it.Notes != "trademark dispute" || it.Strikes != 2 {
		t.Errorf("row mismatch: %+v", it)
	}
	if body.NextCursor != "opaque-next" {
		t.Errorf("next_cursor: want opaque-next, got %q", body.NextCursor)
	}
}

func TestAdminTakedowns_BadLimit(t *testing.T) {
	repo := &fakeAuditLister{}
	r := mountAdminTakedowns(t, repo, "admin@raven.org", "admin@raven.org")
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/marketplace/takedowns?limit=oops", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d, body=%s", w.Code, w.Body.String())
	}
	if repo.called != 0 {
		t.Errorf("repo should not be called on parse error, called=%d", repo.called)
	}
}

func TestAdminTakedowns_InvalidCursorBubbles400(t *testing.T) {
	repo := &fakeAuditLister{
		err: fmt.Errorf("Takedowns.ListAudit: cursor: %w", marketplace.ErrInvalidAuditCursor),
	}
	r := mountAdminTakedowns(t, repo, "admin@raven.org", "admin@raven.org")
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/marketplace/takedowns?cursor=garbage", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d, body=%s", w.Code, w.Body.String())
	}
}

func TestAdminTakedowns_RepoErrorBubbles500(t *testing.T) {
	repo := &fakeAuditLister{err: errors.New("boom")}
	r := mountAdminTakedowns(t, repo, "admin@raven.org", "admin@raven.org")
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/marketplace/takedowns", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

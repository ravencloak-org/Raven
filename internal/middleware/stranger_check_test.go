package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
)

// ── mock StrangerService ──────────────────────────────────────────────────────

type mockStrangerSvcForMiddleware struct {
	upsertFn        func(ctx context.Context, orgID string, req model.UpsertStrangerRequest) (*model.StrangerUser, error)
	flagSuspiciousFn func(ctx context.Context, orgID, strangerID string) error
}

func (m *mockStrangerSvcForMiddleware) Upsert(ctx context.Context, orgID string, req model.UpsertStrangerRequest) (*model.StrangerUser, error) {
	return m.upsertFn(ctx, orgID, req)
}

func (m *mockStrangerSvcForMiddleware) FlagSuspicious(ctx context.Context, orgID, strangerID string) error {
	if m.flagSuspiciousFn != nil {
		return m.flagSuspiciousFn(ctx, orgID, strangerID)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newStrangerCheckRouter(svc middleware.StrangerServiceInterface, valkey *redis.Client, setOrgID bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if setOrgID {
		r.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyOrgID), "org-test")
			c.Next()
		})
	}
	r.Use(middleware.StrangerCheck(svc, valkey))
	r.POST("/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func newMiniredisClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return client, mr
}

func doStrangerCheckRequest(r *gin.Engine, sessionID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/chat/completions", nil)
	if sessionID != "" {
		req.Header.Set("X-Session-ID", sessionID)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestStrangerCheck_NoOrgID verifies that requests without an org context pass through.
func TestStrangerCheck_NoOrgID(t *testing.T) {
	svc := &mockStrangerSvcForMiddleware{
		upsertFn: func(_ context.Context, _ string, _ model.UpsertStrangerRequest) (*model.StrangerUser, error) {
			t.Fatal("upsert should not be called when org is missing")
			return nil, nil
		},
	}
	client, _ := newMiniredisClient(t)
	r := newStrangerCheckRouter(svc, client, false)
	w := doStrangerCheckRequest(r, "sess-1")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestStrangerCheck_MissingSessionID returns 400 when the header is absent.
func TestStrangerCheck_MissingSessionID(t *testing.T) {
	svc := &mockStrangerSvcForMiddleware{
		upsertFn: func(_ context.Context, _ string, _ model.UpsertStrangerRequest) (*model.StrangerUser, error) {
			t.Fatal("upsert should not be called")
			return nil, nil
		},
	}
	client, _ := newMiniredisClient(t)
	r := newStrangerCheckRouter(svc, client, true)
	w := doStrangerCheckRequest(r, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// TestStrangerCheck_ActiveUser_Passes verifies that active strangers pass through.
func TestStrangerCheck_ActiveUser_Passes(t *testing.T) {
	svc := &mockStrangerSvcForMiddleware{
		upsertFn: func(_ context.Context, _ string, _ model.UpsertStrangerRequest) (*model.StrangerUser, error) {
			return &model.StrangerUser{
				ID:      "stranger-1",
				OrgID:   "org-test",
				Status:  model.StrangerStatusActive,
			}, nil
		},
	}
	client, _ := newMiniredisClient(t)
	r := newStrangerCheckRouter(svc, client, true)
	w := doStrangerCheckRequest(r, "sess-active")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestStrangerCheck_BlockedUser_Returns403 verifies that blocked strangers are denied.
func TestStrangerCheck_BlockedUser_Returns403(t *testing.T) {
	svc := &mockStrangerSvcForMiddleware{
		upsertFn: func(_ context.Context, _ string, _ model.UpsertStrangerRequest) (*model.StrangerUser, error) {
			return &model.StrangerUser{
				ID:     "stranger-blocked",
				OrgID:  "org-test",
				Status: model.StrangerStatusBlocked,
			}, nil
		},
	}
	client, _ := newMiniredisClient(t)
	r := newStrangerCheckRouter(svc, client, true)
	w := doStrangerCheckRequest(r, "sess-blocked")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// TestStrangerCheck_BannedUser_Returns403 verifies that banned strangers are denied.
func TestStrangerCheck_BannedUser_Returns403(t *testing.T) {
	svc := &mockStrangerSvcForMiddleware{
		upsertFn: func(_ context.Context, _ string, _ model.UpsertStrangerRequest) (*model.StrangerUser, error) {
			return &model.StrangerUser{
				ID:     "stranger-banned",
				OrgID:  "org-test",
				Status: model.StrangerStatusBanned,
			}, nil
		},
	}
	client, _ := newMiniredisClient(t)
	r := newStrangerCheckRouter(svc, client, true)
	w := doStrangerCheckRequest(r, "sess-banned")
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// TestStrangerCheck_ThrottledUser_WithinLimit_Passes verifies that throttled
// strangers below the RPM cap are admitted.
func TestStrangerCheck_ThrottledUser_WithinLimit_Passes(t *testing.T) {
	rpm := 10
	svc := &mockStrangerSvcForMiddleware{
		upsertFn: func(_ context.Context, _ string, _ model.UpsertStrangerRequest) (*model.StrangerUser, error) {
			return &model.StrangerUser{
				ID:           "stranger-throttled",
				OrgID:        "org-test",
				Status:       model.StrangerStatusThrottled,
				RateLimitRPM: &rpm,
			}, nil
		},
	}
	client, _ := newMiniredisClient(t)
	r := newStrangerCheckRouter(svc, client, true)
	// Send 5 requests — all under the rpm=10 cap.
	for i := 0; i < 5; i++ {
		w := doStrangerCheckRequest(r, "sess-throttled")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

// TestStrangerCheck_ThrottledUser_ExceedsLimit_Returns429 verifies that
// throttled strangers above the RPM cap receive 429.
func TestStrangerCheck_ThrottledUser_ExceedsLimit_Returns429(t *testing.T) {
	rpm := 3
	svc := &mockStrangerSvcForMiddleware{
		upsertFn: func(_ context.Context, _ string, _ model.UpsertStrangerRequest) (*model.StrangerUser, error) {
			return &model.StrangerUser{
				ID:           "stranger-throttled-429",
				OrgID:        "org-test",
				Status:       model.StrangerStatusThrottled,
				RateLimitRPM: &rpm,
			}, nil
		},
	}
	client, _ := newMiniredisClient(t)
	r := newStrangerCheckRouter(svc, client, true)

	// First 3 requests should be admitted.
	for i := 0; i < 3; i++ {
		w := doStrangerCheckRequest(r, "sess-throttled-429")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
	// 4th request must be rejected.
	w := doStrangerCheckRequest(r, "sess-throttled-429")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 4th request, got %d", w.Code)
	}
}

// TestStrangerCheck_SuspiciousBurst_AutoThrottles verifies that an active
// stranger that exceeds the suspiciousThreshold is auto-flagged.
func TestStrangerCheck_SuspiciousBurst_AutoThrottles(t *testing.T) {
	flagged := false
	svc := &mockStrangerSvcForMiddleware{
		upsertFn: func(_ context.Context, _ string, _ model.UpsertStrangerRequest) (*model.StrangerUser, error) {
			return &model.StrangerUser{
				ID:    "stranger-burst",
				OrgID: "org-test",
				// Always returns active so the burst check runs on every request.
				Status: model.StrangerStatusActive,
			}, nil
		},
		flagSuspiciousFn: func(_ context.Context, orgID, strangerID string) error {
			flagged = true
			return nil
		},
	}
	client, _ := newMiniredisClient(t)
	// Pre-seed the burst counter above suspiciousThreshold (30) so the very
	// first test request trips the detection.
	if err := client.Set(context.Background(), "stranger_burst:org-test:sess-burst", "31", 0).Err(); err != nil {
		t.Fatalf("redis Set: %v", err)
	}

	r := newStrangerCheckRouter(svc, client, true)
	// This single request should detect the burst and call FlagSuspicious.
	doStrangerCheckRequest(r, "sess-burst")

	if !flagged {
		t.Error("expected FlagSuspicious to be called when burst exceeds threshold")
	}
}

// TestStrangerCheck_UpsertError_Returns503 verifies fail-closed behaviour when
// the DB is unavailable.
func TestStrangerCheck_UpsertError_Returns503(t *testing.T) {
	svc := &mockStrangerSvcForMiddleware{
		upsertFn: func(_ context.Context, _ string, _ model.UpsertStrangerRequest) (*model.StrangerUser, error) {
			return nil, &mockServiceErr{"db unavailable"}
		},
	}
	client, _ := newMiniredisClient(t)
	r := newStrangerCheckRouter(svc, client, true)
	w := doStrangerCheckRequest(r, "sess-dberr")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// mockServiceErr is a simple error for tests.
type mockServiceErr struct{ msg string }

func (e *mockServiceErr) Error() string { return e.msg }

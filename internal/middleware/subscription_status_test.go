package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
)

type mockSubscriptionChecker struct {
	sub *model.Subscription
	err error
}

func (m *mockSubscriptionChecker) GetActiveSubscription(_ context.Context, _ string) (*model.Subscription, error) {
	return m.sub, m.err
}

func setupTestRouter(checker middleware.SubscriptionStatusChecker) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		// Simulate auth middleware setting org_id.
		c.Set("org_id", "org-test-123")
		c.Next()
	})
	r.Use(middleware.RequireActiveSubscription(checker))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"pong": true})
	})
	return r
}

func TestRequireActiveSubscription_ExpiredReturns402(t *testing.T) {
	checker := &mockSubscriptionChecker{
		sub: &model.Subscription{
			ID:     "sub-1",
			OrgID:  "org-test-123",
			Status: model.SubscriptionStatusExpired,
		},
	}

	r := setupTestRouter(checker)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusPaymentRequired, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "Subscription expired")
}

func TestRequireActiveSubscription_ActiveAllows200(t *testing.T) {
	checker := &mockSubscriptionChecker{
		sub: &model.Subscription{
			ID:     "sub-1",
			OrgID:  "org-test-123",
			Status: model.SubscriptionStatusActive,
		},
	}

	r := setupTestRouter(checker)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireActiveSubscription_TrialingAllows200(t *testing.T) {
	checker := &mockSubscriptionChecker{
		sub: &model.Subscription{
			ID:     "sub-1",
			OrgID:  "org-test-123",
			Status: model.SubscriptionStatusTrialing,
		},
	}

	r := setupTestRouter(checker)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireActiveSubscription_NilSubAllows200(t *testing.T) {
	// Free plan orgs have no subscription record — should pass through.
	checker := &mockSubscriptionChecker{sub: nil}

	r := setupTestRouter(checker)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireActiveSubscription_DBErrorFailsOpen(t *testing.T) {
	// Transient DB errors should fail open (allow the request) to avoid
	// blocking legitimate users when the database is temporarily unavailable.
	checker := &mockSubscriptionChecker{err: context.DeadlineExceeded}

	r := setupTestRouter(checker)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "transient error should fail open")
}

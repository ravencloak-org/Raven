package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

type mockUsageService struct {
	getUsageFn func(ctx context.Context, orgID string) (*model.UsageResponse, error)
}

func (m *mockUsageService) GetUsage(ctx context.Context, orgID string) (*model.UsageResponse, error) {
	return m.getUsageFn(ctx, orgID)
}

func newUsageRouter(svc handler.UsageServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewUsageHandler(svc)

	authed := r.Group("/api/v1/billing")
	authed.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-123")
		c.Next()
	})
	authed.GET("/usage", h.GetUsage)

	return r
}

func TestGetUsage_Success(t *testing.T) {
	svc := &mockUsageService{
		getUsageFn: func(_ context.Context, orgID string) (*model.UsageResponse, error) {
			return &model.UsageResponse{
				Plan:     model.DefaultPlans()[0],
				KBsUsed:  2,
				KBsLimit: 3,
			}, nil
		},
	}
	r := newUsageRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/billing/usage", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var usage model.UsageResponse
	if err := json.Unmarshal(w.Body.Bytes(), &usage); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if usage.KBsUsed != 2 {
		t.Errorf("expected kbs_used 2, got %d", usage.KBsUsed)
	}
}

func TestGetUsage_NoAuth_Returns401(t *testing.T) {
	svc := &mockUsageService{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewUsageHandler(svc)
	r.GET("/api/v1/billing/usage", h.GetUsage)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/billing/usage", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

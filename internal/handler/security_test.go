package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockSecurityService implements handler.SecurityServicer for unit tests.
type mockSecurityService struct {
	createFn          func(ctx context.Context, orgID, userID string, req model.CreateSecurityRuleRequest) (*model.SecurityRule, error)
	getByIDFn         func(ctx context.Context, orgID, ruleID string) (*model.SecurityRule, error)
	listFn            func(ctx context.Context, orgID string) ([]model.SecurityRule, error)
	updateFn          func(ctx context.Context, orgID, ruleID string, req model.UpdateSecurityRuleRequest) (*model.SecurityRule, error)
	deleteFn          func(ctx context.Context, orgID, ruleID string) error
	listEventsFn      func(ctx context.Context, orgID string, limit, offset int) (*model.SecurityEventResponse, error)
	invalidateCacheFn func(ctx context.Context, orgID string)
}

func (m *mockSecurityService) Create(ctx context.Context, orgID, userID string, req model.CreateSecurityRuleRequest) (*model.SecurityRule, error) {
	return m.createFn(ctx, orgID, userID, req)
}
func (m *mockSecurityService) GetByID(ctx context.Context, orgID, ruleID string) (*model.SecurityRule, error) {
	return m.getByIDFn(ctx, orgID, ruleID)
}
func (m *mockSecurityService) List(ctx context.Context, orgID string) ([]model.SecurityRule, error) {
	return m.listFn(ctx, orgID)
}
func (m *mockSecurityService) Update(ctx context.Context, orgID, ruleID string, req model.UpdateSecurityRuleRequest) (*model.SecurityRule, error) {
	return m.updateFn(ctx, orgID, ruleID, req)
}
func (m *mockSecurityService) Delete(ctx context.Context, orgID, ruleID string) error {
	return m.deleteFn(ctx, orgID, ruleID)
}
func (m *mockSecurityService) ListEvents(ctx context.Context, orgID string, limit, offset int) (*model.SecurityEventResponse, error) {
	return m.listEventsFn(ctx, orgID, limit, offset)
}
func (m *mockSecurityService) InvalidateCache(ctx context.Context, orgID string) {
	if m.invalidateCacheFn != nil {
		m.invalidateCacheFn(ctx, orgID)
	}
}

func newSecurityRouter(svc handler.SecurityServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Next()
	})
	h := handler.NewSecurityHandler(svc)
	const base = "/api/v1/orgs/:org_id/security"
	r.POST(base+"/rules", h.CreateRule)
	r.GET(base+"/rules", h.ListRules)
	r.GET(base+"/rules/:rule_id", h.GetRule)
	r.PUT(base+"/rules/:rule_id", h.UpdateRule)
	r.DELETE(base+"/rules/:rule_id", h.DeleteRule)
	r.GET(base+"/events", h.ListEvents)
	r.POST(base+"/rules/invalidate-cache", h.InvalidateRuleCache)
	return r
}

func TestCreateSecurityRule_Success(t *testing.T) {
	svc := &mockSecurityService{
		createFn: func(_ context.Context, orgID, userID string, req model.CreateSecurityRuleRequest) (*model.SecurityRule, error) {
			return &model.SecurityRule{
				ID:       "rule-1",
				OrgID:    orgID,
				Name:     req.Name,
				RuleType: req.RuleType,
				Action:   req.Action,
				IPCIDRs:  req.IPCIDRs,
				IsActive: true,
			}, nil
		},
	}
	r := newSecurityRouter(svc)
	body, _ := json.Marshal(map[string]any{
		"name":      "Block bad IPs",
		"rule_type": "ip_denylist",
		"action":    "block",
		"ip_cidrs":  []string{"10.0.0.0/8"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/security/rules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.SecurityRule
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Name != "Block bad IPs" {
		t.Errorf("expected name 'Block bad IPs', got %q", resp.Name)
	}
}

func TestCreateSecurityRule_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockSecurityService{}
	r := newSecurityRouter(svc)
	// Missing required fields
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/security/rules", bytes.NewBufferString(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestCreateSecurityRule_ServiceError_Returns400(t *testing.T) {
	svc := &mockSecurityService{
		createFn: func(_ context.Context, _, _ string, _ model.CreateSecurityRuleRequest) (*model.SecurityRule, error) {
			return nil, apierror.NewBadRequest("ip_cidrs is required")
		},
	}
	r := newSecurityRouter(svc)
	body, _ := json.Marshal(map[string]any{
		"name":      "Bad rule",
		"rule_type": "ip_denylist",
		"action":    "block",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/security/rules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListSecurityRules_Success(t *testing.T) {
	svc := &mockSecurityService{
		listFn: func(_ context.Context, orgID string) ([]model.SecurityRule, error) {
			return []model.SecurityRule{
				{ID: "rule-1", OrgID: orgID, Name: "Allow office", RuleType: model.SecurityRuleIPAllowlist, IsActive: true},
			}, nil
		},
	}
	r := newSecurityRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/security/rules", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var rules []model.SecurityRule
	if err := json.Unmarshal(w.Body.Bytes(), &rules); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
}

func TestListSecurityRules_ReturnsEmptyArray(t *testing.T) {
	svc := &mockSecurityService{
		listFn: func(_ context.Context, _ string) ([]model.SecurityRule, error) {
			return nil, nil
		},
	}
	r := newSecurityRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/security/rules", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "[]" {
		t.Errorf("expected empty array [], got %s", w.Body.String())
	}
}

func TestGetSecurityRule_Success(t *testing.T) {
	svc := &mockSecurityService{
		getByIDFn: func(_ context.Context, orgID, ruleID string) (*model.SecurityRule, error) {
			return &model.SecurityRule{
				ID:       ruleID,
				OrgID:    orgID,
				Name:     "Block scanners",
				RuleType: model.SecurityRulePatternMatch,
				IsActive: true,
			}, nil
		},
	}
	r := newSecurityRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/security/rules/rule-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSecurityRule_NotFound(t *testing.T) {
	svc := &mockSecurityService{
		getByIDFn: func(_ context.Context, _, _ string) (*model.SecurityRule, error) {
			return nil, apierror.NewNotFound("security rule not found")
		},
	}
	r := newSecurityRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/security/rules/bad-id", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateSecurityRule_Success(t *testing.T) {
	svc := &mockSecurityService{
		updateFn: func(_ context.Context, orgID, ruleID string, _ model.UpdateSecurityRuleRequest) (*model.SecurityRule, error) {
			return &model.SecurityRule{
				ID:       ruleID,
				OrgID:    orgID,
				Name:     "Updated Rule",
				RuleType: model.SecurityRuleIPDenylist,
				IsActive: true,
			}, nil
		},
	}
	r := newSecurityRouter(svc)
	body, _ := json.Marshal(map[string]any{
		"name": "Updated Rule",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/security/rules/rule-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteSecurityRule_Success(t *testing.T) {
	svc := &mockSecurityService{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newSecurityRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/security/rules/rule-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestDeleteSecurityRule_NotFound(t *testing.T) {
	svc := &mockSecurityService{
		deleteFn: func(_ context.Context, _, _ string) error {
			return apierror.NewNotFound("security rule not found")
		},
	}
	r := newSecurityRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/security/rules/bad-id", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListSecurityEvents_Success(t *testing.T) {
	svc := &mockSecurityService{
		listEventsFn: func(_ context.Context, orgID string, limit, offset int) (*model.SecurityEventResponse, error) {
			return &model.SecurityEventResponse{
				Events: []model.SecurityEvent{
					{
						ID:            "evt-1",
						OrgID:         orgID,
						EventType:     "blocked",
						IPAddress:     "10.0.0.1",
						RequestPath:   "/api/v1/chat",
						RequestMethod: "POST",
						CreatedAt:     time.Now(),
					},
				},
				Total: 1,
			}, nil
		},
	}
	r := newSecurityRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/security/events?limit=10&offset=0", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.SecurityEventResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Total)
	}
}

func TestInvalidateCache_Success(t *testing.T) {
	called := false
	svc := &mockSecurityService{
		invalidateCacheFn: func(_ context.Context, orgID string) {
			called = true
			if orgID != "org-abc" {
				t.Errorf("expected orgID org-abc, got %s", orgID)
			}
		},
	}
	r := newSecurityRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/security/rules/invalidate-cache", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
	if !called {
		t.Error("expected InvalidateCache to be called")
	}
}

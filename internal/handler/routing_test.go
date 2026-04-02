package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockRoutingService implements handler.RoutingServicer for unit tests.
type mockRoutingService struct {
	createFn          func(ctx context.Context, orgID string, req model.CreateRoutingRuleRequest, createdBy string) (*model.RoutingRule, error)
	getFn             func(ctx context.Context, orgID, ruleID string) (*model.RoutingRule, error)
	listFn            func(ctx context.Context, orgID string, page, pageSize int) (*model.RoutingRuleListResponse, error)
	updateFn          func(ctx context.Context, orgID, ruleID string, req model.UpdateRoutingRuleRequest) (*model.RoutingRule, error)
	deleteFn          func(ctx context.Context, orgID, ruleID string) error
	resolveFn         func(ctx context.Context, orgID, sourceType, sourceIdentifier string, metadata map[string]any) (*model.ResolveRoutingResponse, error)
	listCatalogFn     func(ctx context.Context, orgID, catalogType string) ([]model.CatalogMetadata, error)
}

func (m *mockRoutingService) Create(ctx context.Context, orgID string, req model.CreateRoutingRuleRequest, createdBy string) (*model.RoutingRule, error) {
	return m.createFn(ctx, orgID, req, createdBy)
}
func (m *mockRoutingService) GetByID(ctx context.Context, orgID, ruleID string) (*model.RoutingRule, error) {
	return m.getFn(ctx, orgID, ruleID)
}
func (m *mockRoutingService) List(ctx context.Context, orgID string, page, pageSize int) (*model.RoutingRuleListResponse, error) {
	return m.listFn(ctx, orgID, page, pageSize)
}
func (m *mockRoutingService) Update(ctx context.Context, orgID, ruleID string, req model.UpdateRoutingRuleRequest) (*model.RoutingRule, error) {
	return m.updateFn(ctx, orgID, ruleID, req)
}
func (m *mockRoutingService) Delete(ctx context.Context, orgID, ruleID string) error {
	return m.deleteFn(ctx, orgID, ruleID)
}
func (m *mockRoutingService) ResolveKBForDocument(ctx context.Context, orgID, sourceType, sourceIdentifier string, metadata map[string]any) (*model.ResolveRoutingResponse, error) {
	return m.resolveFn(ctx, orgID, sourceType, sourceIdentifier, metadata)
}
func (m *mockRoutingService) ListCatalogMetadata(ctx context.Context, orgID, catalogType string) ([]model.CatalogMetadata, error) {
	return m.listCatalogFn(ctx, orgID, catalogType)
}

func newRoutingRouter(svc handler.RoutingServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Next()
	})
	h := handler.NewRoutingHandler(svc)
	const base = "/api/v1/orgs/:org_id/routing-rules"
	r.POST(base, h.Create)
	r.GET(base, h.List)
	r.GET(base+"/:rule_id", h.Get)
	r.PUT(base+"/:rule_id", h.Update)
	r.DELETE(base+"/:rule_id", h.Delete)
	r.POST(base+"/resolve", h.Resolve)
	r.GET("/api/v1/orgs/:org_id/catalog", h.ListCatalog)
	return r
}

func TestCreateRoutingRule_Success(t *testing.T) {
	targetKB := "kb-123"
	svc := &mockRoutingService{
		createFn: func(_ context.Context, orgID string, req model.CreateRoutingRuleRequest, createdBy string) (*model.RoutingRule, error) {
			return &model.RoutingRule{
				ID:          "rule-1",
				OrgID:       orgID,
				Name:        req.Name,
				SourceType:  req.SourceType,
				RoutingMode: req.RoutingMode,
				TargetKBID:  req.TargetKBID,
				Priority:    req.Priority,
				IsActive:    true,
			}, nil
		},
	}
	r := newRoutingRouter(svc)
	body, _ := json.Marshal(map[string]any{
		"name":         "Test Rule",
		"source_type":  "upload",
		"routing_mode": "static",
		"target_kb_id": targetKB,
		"priority":     10,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/routing-rules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp model.RoutingRule
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Name != "Test Rule" {
		t.Errorf("expected name 'Test Rule', got %q", resp.Name)
	}
}

func TestCreateRoutingRule_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockRoutingService{}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	// Missing required fields
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/routing-rules", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestCreateRoutingRule_ServiceError_Returns400(t *testing.T) {
	svc := &mockRoutingService{
		createFn: func(_ context.Context, _ string, _ model.CreateRoutingRuleRequest, _ string) (*model.RoutingRule, error) {
			return nil, apierror.NewBadRequest("static routing mode requires target_kb_id")
		},
	}
	r := newRoutingRouter(svc)
	body, _ := json.Marshal(map[string]any{
		"name":         "Bad Rule",
		"source_type":  "upload",
		"routing_mode": "static",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/routing-rules", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetRoutingRule_Success(t *testing.T) {
	svc := &mockRoutingService{
		getFn: func(_ context.Context, orgID, ruleID string) (*model.RoutingRule, error) {
			return &model.RoutingRule{ID: ruleID, OrgID: orgID, Name: "Test Rule"}, nil
		},
	}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/routing-rules/rule-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetRoutingRule_NotFound_Returns404(t *testing.T) {
	svc := &mockRoutingService{
		getFn: func(_ context.Context, _, _ string) (*model.RoutingRule, error) {
			return nil, apierror.NewNotFound("routing rule not found")
		},
	}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/routing-rules/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListRoutingRules_Success(t *testing.T) {
	svc := &mockRoutingService{
		listFn: func(_ context.Context, _ string, page, pageSize int) (*model.RoutingRuleListResponse, error) {
			return &model.RoutingRuleListResponse{
				Data:     []model.RoutingRule{},
				Total:    0,
				Page:     page,
				PageSize: pageSize,
			}, nil
		},
	}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/routing-rules", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp model.RoutingRuleListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil empty array")
	}
}

func TestListRoutingRules_WithPagination(t *testing.T) {
	svc := &mockRoutingService{
		listFn: func(_ context.Context, _ string, page, pageSize int) (*model.RoutingRuleListResponse, error) {
			return &model.RoutingRuleListResponse{
				Data:     []model.RoutingRule{},
				Total:    50,
				Page:     page,
				PageSize: pageSize,
			}, nil
		},
	}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/routing-rules?page=2&page_size=10", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp model.RoutingRuleListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Page != 2 {
		t.Errorf("expected page 2, got %d", resp.Page)
	}
	if resp.PageSize != 10 {
		t.Errorf("expected page_size 10, got %d", resp.PageSize)
	}
}

func TestUpdateRoutingRule_Success(t *testing.T) {
	svc := &mockRoutingService{
		updateFn: func(_ context.Context, orgID, ruleID string, _ model.UpdateRoutingRuleRequest) (*model.RoutingRule, error) {
			return &model.RoutingRule{ID: ruleID, OrgID: orgID, Name: "Updated Rule"}, nil
		},
	}
	r := newRoutingRouter(svc)
	body, _ := json.Marshal(map[string]string{"name": "Updated Rule"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/routing-rules/rule-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateRoutingRule_NotFound_Returns404(t *testing.T) {
	svc := &mockRoutingService{
		updateFn: func(_ context.Context, _, _ string, _ model.UpdateRoutingRuleRequest) (*model.RoutingRule, error) {
			return nil, apierror.NewNotFound("routing rule not found")
		},
	}
	r := newRoutingRouter(svc)
	body, _ := json.Marshal(map[string]string{"name": "Updated"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/routing-rules/bad-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteRoutingRule_Success(t *testing.T) {
	svc := &mockRoutingService{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/routing-rules/rule-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestDeleteRoutingRule_NotFound_Returns404(t *testing.T) {
	svc := &mockRoutingService{
		deleteFn: func(_ context.Context, _, _ string) error {
			return apierror.NewNotFound("routing rule not found")
		},
	}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/routing-rules/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestResolveRouting_Success(t *testing.T) {
	svc := &mockRoutingService{
		resolveFn: func(_ context.Context, _ string, sourceType, sourceIdentifier string, _ map[string]any) (*model.ResolveRoutingResponse, error) {
			return &model.ResolveRoutingResponse{
				KnowledgeBaseID: "kb-123",
				RuleName:        "Test Rule",
				RuleID:          "rule-1",
			}, nil
		},
	}
	r := newRoutingRouter(svc)
	body, _ := json.Marshal(map[string]any{
		"source_type":       "upload",
		"source_identifier": "my-table",
		"metadata":          map[string]any{"department": "engineering"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/routing-rules/resolve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp model.ResolveRoutingResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.KnowledgeBaseID != "kb-123" {
		t.Errorf("expected kb-123, got %q", resp.KnowledgeBaseID)
	}
}

func TestResolveRouting_NoRuleFound_Returns404(t *testing.T) {
	svc := &mockRoutingService{
		resolveFn: func(_ context.Context, _, _, _ string, _ map[string]any) (*model.ResolveRoutingResponse, error) {
			return nil, apierror.NewNotFound("no routing rule found for source")
		},
	}
	r := newRoutingRouter(svc)
	body, _ := json.Marshal(map[string]any{
		"source_type": "unknown",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/routing-rules/resolve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveRouting_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockRoutingService{}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	// Missing required source_type
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/routing-rules/resolve", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestListCatalog_Success(t *testing.T) {
	svc := &mockRoutingService{
		listCatalogFn: func(_ context.Context, _, _ string) ([]model.CatalogMetadata, error) {
			return nil, nil
		},
	}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/catalog", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := strings.TrimSpace(w.Body.String())
	if body != "[]" {
		t.Errorf("expected empty array '[]', got %q", body)
	}
}

func TestListCatalog_WithFilter(t *testing.T) {
	svc := &mockRoutingService{
		listCatalogFn: func(_ context.Context, _, catalogType string) ([]model.CatalogMetadata, error) {
			if catalogType != "dbt" {
				t.Errorf("expected catalog_type 'dbt', got %q", catalogType)
			}
			return []model.CatalogMetadata{}, nil
		},
	}
	r := newRoutingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/catalog?catalog_type=dbt", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

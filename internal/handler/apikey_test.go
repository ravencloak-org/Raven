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

// mockApiKeyService implements handler.ApiKeyServicer for unit tests.
type mockApiKeyService struct {
	createFn func(ctx context.Context, orgID, wsID, kbID, userID string, req model.CreateApiKeyRequest) (*model.CreateApiKeyResponse, error)
	listFn   func(ctx context.Context, orgID, kbID string) ([]model.ApiKey, error)
	revokeFn func(ctx context.Context, orgID, id string) error
}

func (m *mockApiKeyService) Create(ctx context.Context, orgID, wsID, kbID, userID string, req model.CreateApiKeyRequest) (*model.CreateApiKeyResponse, error) {
	return m.createFn(ctx, orgID, wsID, kbID, userID, req)
}

func (m *mockApiKeyService) List(ctx context.Context, orgID, kbID string) ([]model.ApiKey, error) {
	return m.listFn(ctx, orgID, kbID)
}

func (m *mockApiKeyService) Revoke(ctx context.Context, orgID, id string) error {
	return m.revokeFn(ctx, orgID, id)
}

func newApiKeyRouter(svc handler.ApiKeyServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Set(string(middleware.ContextKeyWorkspaceRole), "admin")
		c.Next()
	})
	h := handler.NewApiKeyHandler(svc)
	const base = "/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/api-keys"
	r.POST(base, h.Create)
	r.GET(base, h.List)
	r.DELETE(base+"/:key_id", h.Revoke)
	return r
}

func TestCreateApiKey_Success(t *testing.T) {
	svc := &mockApiKeyService{
		createFn: func(_ context.Context, orgID, wsID, kbID, userID string, req model.CreateApiKeyRequest) (*model.CreateApiKeyResponse, error) {
			return &model.CreateApiKeyResponse{
				ApiKey: model.ApiKey{
					ID:              "key-1",
					OrgID:           orgID,
					WorkspaceID:     wsID,
					KnowledgeBaseID: kbID,
					Name:            req.Name,
					KeyPrefix:       "abcdef12",
					RateLimit:       60,
					Status:          model.ApiKeyStatusActive,
					CreatedBy:       userID,
					CreatedAt:       time.Now(),
				},
				RawKey: "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			}, nil
		},
	}
	r := newApiKeyRouter(svc)
	body, _ := json.Marshal(map[string]string{"name": "Test Key"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.CreateApiKeyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.RawKey == "" {
		t.Error("expected raw_key in response")
	}
	if resp.Name != "Test Key" {
		t.Errorf("expected name 'Test Key', got %q", resp.Name)
	}
}

func TestCreateApiKey_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockApiKeyService{}
	r := newApiKeyRouter(svc)
	w := httptest.NewRecorder()
	// Name is required with min=2
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/api-keys", bytes.NewBufferString(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestCreateApiKey_ServiceError_Returns500(t *testing.T) {
	svc := &mockApiKeyService{
		createFn: func(_ context.Context, _, _, _, _ string, _ model.CreateApiKeyRequest) (*model.CreateApiKeyResponse, error) {
			return nil, apierror.NewInternal("db error")
		},
	}
	r := newApiKeyRouter(svc)
	body, _ := json.Marshal(map[string]string{"name": "Test Key"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListApiKeys_Success(t *testing.T) {
	svc := &mockApiKeyService{
		listFn: func(_ context.Context, orgID, kbID string) ([]model.ApiKey, error) {
			return []model.ApiKey{
				{ID: "key-1", OrgID: orgID, KnowledgeBaseID: kbID, Name: "Key 1", KeyPrefix: "abcdef12", Status: model.ApiKeyStatusActive},
			}, nil
		},
	}
	r := newApiKeyRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/api-keys", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var keys []model.ApiKey
	if err := json.Unmarshal(w.Body.Bytes(), &keys); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}
}

func TestListApiKeys_ReturnsEmptyArray(t *testing.T) {
	svc := &mockApiKeyService{
		listFn: func(_ context.Context, _, _ string) ([]model.ApiKey, error) {
			return nil, nil
		},
	}
	r := newApiKeyRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/api-keys", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "[]" {
		t.Errorf("expected empty array [], got %s", w.Body.String())
	}
}

func TestRevokeApiKey_Success(t *testing.T) {
	svc := &mockApiKeyService{
		revokeFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newApiKeyRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/api-keys/key-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestRevokeApiKey_NotFound_Returns404(t *testing.T) {
	svc := &mockApiKeyService{
		revokeFn: func(_ context.Context, _, _ string) error {
			return apierror.NewNotFound("api key not found or already revoked")
		},
	}
	r := newApiKeyRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/api-keys/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

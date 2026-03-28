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

// mockLLMProviderService implements handler.LLMProviderServicer for unit tests.
type mockLLMProviderService struct {
	createFn     func(ctx context.Context, orgID, userID string, req model.CreateLLMProviderRequest) (*model.LLMProviderResponse, error)
	getFn        func(ctx context.Context, orgID, configID string) (*model.LLMProviderResponse, error)
	listFn       func(ctx context.Context, orgID string) ([]model.LLMProviderResponse, error)
	updateFn     func(ctx context.Context, orgID, configID string, req model.UpdateLLMProviderRequest) (*model.LLMProviderResponse, error)
	deleteFn     func(ctx context.Context, orgID, configID string) error
	setDefaultFn func(ctx context.Context, orgID, configID string) error
}

func (m *mockLLMProviderService) Create(ctx context.Context, orgID, userID string, req model.CreateLLMProviderRequest) (*model.LLMProviderResponse, error) {
	return m.createFn(ctx, orgID, userID, req)
}
func (m *mockLLMProviderService) GetByID(ctx context.Context, orgID, configID string) (*model.LLMProviderResponse, error) {
	return m.getFn(ctx, orgID, configID)
}
func (m *mockLLMProviderService) List(ctx context.Context, orgID string) ([]model.LLMProviderResponse, error) {
	return m.listFn(ctx, orgID)
}
func (m *mockLLMProviderService) Update(ctx context.Context, orgID, configID string, req model.UpdateLLMProviderRequest) (*model.LLMProviderResponse, error) {
	return m.updateFn(ctx, orgID, configID, req)
}
func (m *mockLLMProviderService) Delete(ctx context.Context, orgID, configID string) error {
	return m.deleteFn(ctx, orgID, configID)
}
func (m *mockLLMProviderService) SetDefault(ctx context.Context, orgID, configID string) error {
	return m.setDefaultFn(ctx, orgID, configID)
}

func newLLMProviderRouter(svc handler.LLMProviderServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Next()
	})
	h := handler.NewLLMProviderHandler(svc)
	const base = "/api/v1/orgs/:org_id/llm-providers"
	r.POST(base, h.Create)
	r.GET(base, h.List)
	r.GET(base+"/:provider_id", h.Get)
	r.PUT(base+"/:provider_id", h.Update)
	r.DELETE(base+"/:provider_id", h.Delete)
	r.PUT(base+"/:provider_id/default", h.SetDefault)
	return r
}

func TestCreateLLMProvider_Success(t *testing.T) {
	svc := &mockLLMProviderService{
		createFn: func(_ context.Context, orgID, userID string, req model.CreateLLMProviderRequest) (*model.LLMProviderResponse, error) {
			return &model.LLMProviderResponse{
				ID:          "prov-1",
				OrgID:       orgID,
				Provider:    req.Provider,
				DisplayName: req.DisplayName,
				APIKeyHint:  "...cret",
				IsDefault:   false,
				Status:      model.ProviderStatusActive,
			}, nil
		},
	}
	r := newLLMProviderRouter(svc)
	body, _ := json.Marshal(map[string]string{
		"provider":     "openai",
		"display_name": "My OpenAI",
		"api_key":      "sk-testsecret",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/llm-providers", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.LLMProviderResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.DisplayName != "My OpenAI" {
		t.Errorf("expected display_name 'My OpenAI', got %q", resp.DisplayName)
	}
}

func TestCreateLLMProvider_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockLLMProviderService{}
	r := newLLMProviderRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/llm-providers", bytes.NewBufferString(`{"provider":"openai"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetLLMProvider_Success(t *testing.T) {
	svc := &mockLLMProviderService{
		getFn: func(_ context.Context, orgID, configID string) (*model.LLMProviderResponse, error) {
			return &model.LLMProviderResponse{
				ID:          configID,
				OrgID:       orgID,
				Provider:    model.LLMProviderOpenAI,
				DisplayName: "Test",
				APIKeyHint:  "...abcd",
				Status:      model.ProviderStatusActive,
			}, nil
		},
	}
	r := newLLMProviderRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/llm-providers/prov-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetLLMProvider_NotFound_Returns404(t *testing.T) {
	svc := &mockLLMProviderService{
		getFn: func(_ context.Context, _, _ string) (*model.LLMProviderResponse, error) {
			return nil, apierror.NewNotFound("LLM provider config not found")
		},
	}
	r := newLLMProviderRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/llm-providers/bad-id", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListLLMProviders_ReturnsEmptyArray(t *testing.T) {
	svc := &mockLLMProviderService{
		listFn: func(_ context.Context, _ string) ([]model.LLMProviderResponse, error) {
			return nil, nil
		},
	}
	r := newLLMProviderRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/llm-providers", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	// Verify it's an empty JSON array, not null.
	body := strings.TrimSpace(w.Body.String())
	if body != "[]" {
		t.Errorf("expected empty array '[]', got %q", body)
	}
}

func TestUpdateLLMProvider_Success(t *testing.T) {
	svc := &mockLLMProviderService{
		updateFn: func(_ context.Context, orgID, configID string, _ model.UpdateLLMProviderRequest) (*model.LLMProviderResponse, error) {
			return &model.LLMProviderResponse{
				ID:          configID,
				OrgID:       orgID,
				Provider:    model.LLMProviderOpenAI,
				DisplayName: "Updated",
				APIKeyHint:  "...wxyz",
				Status:      model.ProviderStatusActive,
			}, nil
		},
	}
	r := newLLMProviderRouter(svc)
	body, _ := json.Marshal(map[string]string{"display_name": "Updated"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/llm-providers/prov-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteLLMProvider_Success(t *testing.T) {
	svc := &mockLLMProviderService{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newLLMProviderRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/llm-providers/prov-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestSetDefaultLLMProvider_Success(t *testing.T) {
	svc := &mockLLMProviderService{
		setDefaultFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newLLMProviderRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/llm-providers/prov-1/default", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

// Security: verify that API responses NEVER contain encrypted or plaintext keys.
func TestLLMProviderResponse_NoKeyLeakage(t *testing.T) {
	svc := &mockLLMProviderService{
		getFn: func(_ context.Context, orgID, configID string) (*model.LLMProviderResponse, error) {
			return &model.LLMProviderResponse{
				ID:          configID,
				OrgID:       orgID,
				Provider:    model.LLMProviderOpenAI,
				DisplayName: "Test",
				APIKeyHint:  "...cret",
				Status:      model.ProviderStatusActive,
			}, nil
		},
	}
	r := newLLMProviderRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/llm-providers/prov-1", nil)
	r.ServeHTTP(w, req)

	body := w.Body.String()

	// Verify the response does not contain any key-related fields that would leak data.
	for _, forbidden := range []string{"api_key_encrypted", "api_key_iv", "\"api_key\""} {
		if strings.Contains(body, forbidden) {
			t.Errorf("response body contains forbidden field %q: %s", forbidden, body)
		}
	}

	// Verify the hint IS present.
	if !strings.Contains(body, "api_key_hint") {
		t.Error("response body should contain api_key_hint")
	}
}

// Security: verify the internal LLMProviderConfig model does not leak keys via JSON.
func TestLLMProviderConfig_JSONOmitsEncryptedFields(t *testing.T) {
	cfg := model.LLMProviderConfig{
		ID:              "test-id",
		OrgID:           "org-1",
		Provider:        model.LLMProviderOpenAI,
		DisplayName:     "Test",
		APIKeyEncrypted: []byte("encrypted-data"),
		APIKeyIV:        []byte("iv-data"),
		APIKeyHint:      "...hint",
		Status:          model.ProviderStatusActive,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	body := string(data)
	for _, forbidden := range []string{"api_key_encrypted", "api_key_iv"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("JSON output of LLMProviderConfig contains %q — encrypted key data must not be serialised", forbidden)
		}
	}
}

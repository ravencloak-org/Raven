package handler_test

import (
	"bytes"
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

// mockUserService implements handler.UserServicer for unit tests.
type mockUserService struct {
	getMeFn               func(ctx context.Context, sub string) (*model.User, error)
	updateMeFn            func(ctx context.Context, userID string, req model.UpdateUserRequest) (*model.User, error)
	getByIDFn             func(ctx context.Context, userID string) (*model.User, error)
	handleKeycloakEventFn func(ctx context.Context, event model.KeycloakWebhookEvent) error
	deleteMeFn            func(ctx context.Context, userID string) error
}

func (m *mockUserService) GetMe(ctx context.Context, sub string) (*model.User, error) {
	return m.getMeFn(ctx, sub)
}
func (m *mockUserService) UpdateMe(ctx context.Context, userID string, req model.UpdateUserRequest) (*model.User, error) {
	return m.updateMeFn(ctx, userID, req)
}
func (m *mockUserService) GetByID(ctx context.Context, userID string) (*model.User, error) {
	return m.getByIDFn(ctx, userID)
}
func (m *mockUserService) HandleKeycloakEvent(ctx context.Context, event model.KeycloakWebhookEvent) error {
	return m.handleKeycloakEventFn(ctx, event)
}
func (m *mockUserService) DeleteMe(ctx context.Context, userID string) error {
	return m.deleteMeFn(ctx, userID)
}

func newUserRouter(svc handler.UserServicer, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), userID)
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	h := handler.NewUserHandler(svc)
	r.GET("/api/v1/me", h.GetMe)
	r.PUT("/api/v1/me", h.UpdateMe)
	r.DELETE("/api/v1/me", h.DeleteMe)
	r.GET("/api/v1/users/:user_id", h.GetUser)
	r.POST("/api/v1/internal/keycloak-webhook", h.KeycloakWebhook)
	return r
}

func TestGetMe_Success(t *testing.T) {
	svc := &mockUserService{
		getMeFn: func(_ context.Context, sub string) (*model.User, error) {
			return &model.User{ID: "user-1", KeycloakSub: sub, Email: "alice@example.com"}, nil
		},
	}
	r := newUserRouter(svc, "kc-sub-alice")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/me", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp model.User
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Email != "alice@example.com" {
		t.Errorf("unexpected email: %s", resp.Email)
	}
}

func TestGetMe_NotFound_Returns404(t *testing.T) {
	svc := &mockUserService{
		getMeFn: func(_ context.Context, _ string) (*model.User, error) {
			return nil, apierror.NewNotFound("user not found")
		},
	}
	r := newUserRouter(svc, "kc-sub-unknown")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/me", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdateMe_Success(t *testing.T) {
	name := "Alice Updated"
	svc := &mockUserService{
		updateMeFn: func(_ context.Context, _ string, req model.UpdateUserRequest) (*model.User, error) {
			return &model.User{ID: "user-1", DisplayName: *req.DisplayName}, nil
		},
	}
	r := newUserRouter(svc, "user-1")

	body, _ := json.Marshal(map[string]string{"display_name": name})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/me", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteMe_Success(t *testing.T) {
	svc := &mockUserService{
		deleteMeFn: func(_ context.Context, _ string) error { return nil },
	}
	r := newUserRouter(svc, "user-1")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/me", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

// TestKeycloakWebhook_Register tests the webhook with a mocked REGISTER event.
// NOTE: Live Keycloak SPI integration test deferred — see issue #26 PR description.
func TestKeycloakWebhook_Register(t *testing.T) {
	svc := &mockUserService{
		handleKeycloakEventFn: func(_ context.Context, event model.KeycloakWebhookEvent) error {
			if event.Type != "REGISTER" {
				return apierror.NewBadRequest("unexpected event type: " + event.Type)
			}
			return nil
		},
	}
	r := newUserRouter(svc, "")

	payload := model.KeycloakWebhookEvent{
		Type:    "REGISTER",
		RealmID: "raven",
		UserID:  "kc-sub-new",
		OrgID:   "org-123",
		Email:   "new@example.com",
	}
	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/keycloak-webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKeycloakWebhook_DeleteAccount(t *testing.T) {
	svc := &mockUserService{
		handleKeycloakEventFn: func(_ context.Context, event model.KeycloakWebhookEvent) error {
			if event.Type != "DELETE_ACCOUNT" {
				return apierror.NewBadRequest("unexpected event type")
			}
			return nil
		},
	}
	r := newUserRouter(svc, "")

	payload := model.KeycloakWebhookEvent{
		Type:    "DELETE_ACCOUNT",
		RealmID: "raven",
		UserID:  "kc-sub-del",
		OrgID:   "org-123",
	}
	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/keycloak-webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKeycloakWebhook_InvalidPayload_Returns400(t *testing.T) {
	svc := &mockUserService{}
	r := newUserRouter(svc, "")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/keycloak-webhook", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

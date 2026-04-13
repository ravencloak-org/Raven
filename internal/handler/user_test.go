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
	getByExternalIDFn func(ctx context.Context, externalID string) (*model.User, error)
	updateMeFn        func(ctx context.Context, userID string, req model.UpdateUserRequest) (*model.User, error)
	getByIDFn         func(ctx context.Context, userID string) (*model.User, error)
	deleteMeFn        func(ctx context.Context, userID string) error
}

func (m *mockUserService) GetByExternalID(ctx context.Context, externalID string) (*model.User, error) {
	if m.getByExternalIDFn != nil {
		return m.getByExternalIDFn(ctx, externalID)
	}
	return nil, nil
}
func (m *mockUserService) UpdateMe(ctx context.Context, userID string, req model.UpdateUserRequest) (*model.User, error) {
	return m.updateMeFn(ctx, userID, req)
}
func (m *mockUserService) GetByID(ctx context.Context, userID string) (*model.User, error) {
	return m.getByIDFn(ctx, userID)
}
func (m *mockUserService) DeleteMe(ctx context.Context, userID string) error {
	return m.deleteMeFn(ctx, userID)
}

func newUserRouter(svc handler.UserServicer, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyExternalID), userID)
		c.Set(string(middleware.ContextKeyUserID), userID)
		c.Set(string(middleware.ContextKeyOrgRole), "member")
		c.Next()
	})
	h := handler.NewUserHandler(svc)
	r.GET("/api/v1/me", h.GetMe)
	r.PUT("/api/v1/me", h.UpdateMe)
	r.DELETE("/api/v1/me", h.DeleteMe)
	r.GET("/api/v1/users/:user_id", h.GetUser)
	return r
}

func TestGetMe_Success(t *testing.T) {
	svc := &mockUserService{
		getByExternalIDFn: func(_ context.Context, externalID string) (*model.User, error) {
			return &model.User{ID: "user-1", ExternalID: externalID, Email: "alice@example.com"}, nil
		},
	}
	r := newUserRouter(svc, "zitadel-sub-alice")

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
		getByExternalIDFn: func(_ context.Context, _ string) (*model.User, error) {
			return nil, apierror.NewNotFound("user not found")
		},
	}
	r := newUserRouter(svc, "zitadel-sub-unknown")

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

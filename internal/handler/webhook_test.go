package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWebhookService implements handler.WebhookServicer for unit tests.
type mockWebhookService struct {
	createFn         func(ctx context.Context, orgID, userID string, req model.CreateWebhookRequest) (*model.WebhookConfig, error)
	getByIDFn        func(ctx context.Context, orgID, id string) (*model.WebhookConfig, error)
	listFn           func(ctx context.Context, orgID string) ([]model.WebhookConfig, error)
	updateFn         func(ctx context.Context, orgID, id string, req model.UpdateWebhookRequest) (*model.WebhookConfig, error)
	deleteFn         func(ctx context.Context, orgID, id string) error
	listDeliveriesFn func(ctx context.Context, orgID, webhookID string, limit int) ([]model.WebhookDelivery, error)
}

func (m *mockWebhookService) Create(ctx context.Context, orgID, userID string, req model.CreateWebhookRequest) (*model.WebhookConfig, error) {
	if m.createFn == nil {
		return nil, errors.New("mockWebhookService.createFn not set")
	}
	return m.createFn(ctx, orgID, userID, req)
}
func (m *mockWebhookService) GetByID(ctx context.Context, orgID, id string) (*model.WebhookConfig, error) {
	if m.getByIDFn == nil {
		return nil, errors.New("mockWebhookService.getByIDFn not set")
	}
	return m.getByIDFn(ctx, orgID, id)
}
func (m *mockWebhookService) List(ctx context.Context, orgID string) ([]model.WebhookConfig, error) {
	if m.listFn == nil {
		return nil, errors.New("mockWebhookService.listFn not set")
	}
	return m.listFn(ctx, orgID)
}
func (m *mockWebhookService) Update(ctx context.Context, orgID, id string, req model.UpdateWebhookRequest) (*model.WebhookConfig, error) {
	if m.updateFn == nil {
		return nil, errors.New("mockWebhookService.updateFn not set")
	}
	return m.updateFn(ctx, orgID, id, req)
}
func (m *mockWebhookService) Delete(ctx context.Context, orgID, id string) error {
	if m.deleteFn == nil {
		return errors.New("mockWebhookService.deleteFn not set")
	}
	return m.deleteFn(ctx, orgID, id)
}
func (m *mockWebhookService) ListDeliveries(ctx context.Context, orgID, webhookID string, limit int) ([]model.WebhookDelivery, error) {
	if m.listDeliveriesFn == nil {
		return nil, errors.New("mockWebhookService.listDeliveriesFn not set")
	}
	return m.listDeliveriesFn(ctx, orgID, webhookID, limit)
}

var testOrgID = uuid.New().String()
var testWebhookID = uuid.New().String()

func newWebhookRouter(svc handler.WebhookServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Set(string(middleware.ContextKeyOrgID), testOrgID)
		c.Next()
	})
	h := handler.NewWebhookHandler(svc)
	base := "/api/v1/orgs/:org_id/webhooks"
	r.POST(base, h.Create)
	r.GET(base, h.List)
	r.GET(base+"/:id", h.Get)
	r.PUT(base+"/:id", h.Update)
	r.DELETE(base+"/:id", h.Delete)
	r.GET(base+"/:id/deliveries", h.ListDeliveries)
	return r
}

func TestWebhookHandler_Create_Success(t *testing.T) {
	maxRetries := 3
	svc := &mockWebhookService{
		createFn: func(_ context.Context, orgID, _ string, _ model.CreateWebhookRequest) (*model.WebhookConfig, error) {
			return &model.WebhookConfig{ID: testWebhookID, OrgID: orgID, Name: "My Webhook", Status: model.WebhookStatusActive}, nil
		},
	}
	r := newWebhookRouter(svc)

	body, _ := json.Marshal(model.CreateWebhookRequest{
		Name:       "My Webhook",
		URL:        "https://example.com/hook",
		Secret:     "supersecret123",
		Events:     []string{"lead.generated"},
		MaxRetries: &maxRetries,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/"+testOrgID+"/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp model.WebhookConfig
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, testWebhookID, resp.ID)
}

func TestWebhookHandler_Create_InvalidOrgID_Returns400(t *testing.T) {
	svc := &mockWebhookService{}
	r := newWebhookRouter(svc)

	body, _ := json.Marshal(model.CreateWebhookRequest{
		Name:   "My Webhook",
		URL:    "https://example.com/hook",
		Secret: "supersecret123",
		Events: []string{"lead.generated"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/not-a-uuid/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookHandler_Create_UnsupportedEvent_Returns400(t *testing.T) {
	svc := &mockWebhookService{}
	r := newWebhookRouter(svc)

	body, _ := json.Marshal(model.CreateWebhookRequest{
		Name:   "My Webhook",
		URL:    "https://example.com/hook",
		Secret: "supersecret123",
		Events: []string{"unknown.event"},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/"+testOrgID+"/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookHandler_Create_ExcessiveRetries_Returns400(t *testing.T) {
	svc := &mockWebhookService{}
	r := newWebhookRouter(svc)

	tooMany := 100
	body, _ := json.Marshal(model.CreateWebhookRequest{
		Name:       "My Webhook",
		URL:        "https://example.com/hook",
		Secret:     "supersecret123",
		Events:     []string{"lead.generated"},
		MaxRetries: &tooMany,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/"+testOrgID+"/webhooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookHandler_List_Success(t *testing.T) {
	svc := &mockWebhookService{
		listFn: func(_ context.Context, orgID string) ([]model.WebhookConfig, error) {
			return []model.WebhookConfig{
				{ID: testWebhookID, OrgID: orgID, Name: "Hook 1", Status: model.WebhookStatusActive},
			}, nil
		},
	}
	r := newWebhookRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/"+testOrgID+"/webhooks", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []model.WebhookConfig
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

func TestWebhookHandler_List_Empty_ReturnsEmptyArray(t *testing.T) {
	svc := &mockWebhookService{
		listFn: func(_ context.Context, _ string) ([]model.WebhookConfig, error) {
			return nil, nil
		},
	}
	r := newWebhookRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/"+testOrgID+"/webhooks", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "[]")
}

func TestWebhookHandler_Get_Success(t *testing.T) {
	svc := &mockWebhookService{
		getByIDFn: func(_ context.Context, orgID, id string) (*model.WebhookConfig, error) {
			return &model.WebhookConfig{ID: id, OrgID: orgID, Name: "Hook 1"}, nil
		},
	}
	r := newWebhookRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/"+testOrgID+"/webhooks/"+testWebhookID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebhookHandler_Get_InvalidID_Returns400(t *testing.T) {
	svc := &mockWebhookService{}
	r := newWebhookRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/"+testOrgID+"/webhooks/not-a-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookHandler_Get_NotFound_Returns404(t *testing.T) {
	svc := &mockWebhookService{
		getByIDFn: func(_ context.Context, _, _ string) (*model.WebhookConfig, error) {
			return nil, apierror.NewNotFound("webhook not found")
		},
	}
	r := newWebhookRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/"+testOrgID+"/webhooks/"+testWebhookID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebhookHandler_Update_Success(t *testing.T) {
	newName := "Updated Hook"
	svc := &mockWebhookService{
		updateFn: func(_ context.Context, orgID, id string, _ model.UpdateWebhookRequest) (*model.WebhookConfig, error) {
			return &model.WebhookConfig{ID: id, OrgID: orgID, Name: newName}, nil
		},
	}
	r := newWebhookRouter(svc)

	body, _ := json.Marshal(model.UpdateWebhookRequest{Name: &newName})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/"+testOrgID+"/webhooks/"+testWebhookID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebhookHandler_Update_InvalidStatus_Returns400(t *testing.T) {
	svc := &mockWebhookService{}
	r := newWebhookRouter(svc)

	badStatus := model.WebhookStatus("invalid_status")
	body, _ := json.Marshal(model.UpdateWebhookRequest{Status: &badStatus})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/"+testOrgID+"/webhooks/"+testWebhookID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebhookHandler_Delete_Success(t *testing.T) {
	svc := &mockWebhookService{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newWebhookRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/"+testOrgID+"/webhooks/"+testWebhookID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestWebhookHandler_Delete_NotFound_Returns404(t *testing.T) {
	svc := &mockWebhookService{
		deleteFn: func(_ context.Context, _, _ string) error {
			return apierror.NewNotFound("webhook not found")
		},
	}
	r := newWebhookRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/"+testOrgID+"/webhooks/"+testWebhookID, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebhookHandler_ListDeliveries_Success(t *testing.T) {
	svc := &mockWebhookService{
		listDeliveriesFn: func(_ context.Context, _, _ string, _ int) ([]model.WebhookDelivery, error) {
			return []model.WebhookDelivery{
				{ID: "del-1", WebhookID: testWebhookID, EventType: "lead.generated", Success: true},
			}, nil
		},
	}
	r := newWebhookRouter(svc)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/"+testOrgID+"/webhooks/"+testWebhookID+"/deliveries", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []model.WebhookDelivery
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
	assert.True(t, resp[0].Success)
}

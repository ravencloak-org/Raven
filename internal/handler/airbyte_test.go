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

// mockAirbyteService implements handler.AirbyteServicer for unit tests.
type mockAirbyteService struct {
	createFn     func(ctx context.Context, orgID, userID string, req model.CreateConnectorRequest) (*model.ConnectorResponse, error)
	getFn        func(ctx context.Context, orgID, connectorID string) (*model.ConnectorResponse, error)
	listFn       func(ctx context.Context, orgID string, page, pageSize int) (*model.ConnectorListResponse, error)
	updateFn     func(ctx context.Context, orgID, connectorID string, req model.UpdateConnectorRequest) (*model.ConnectorResponse, error)
	deleteFn     func(ctx context.Context, orgID, connectorID string) error
	triggerFn    func(ctx context.Context, orgID, connectorID string) error
	historyFn    func(ctx context.Context, orgID, connectorID string, limit int) ([]model.SyncHistoryResponse, error)
}

func (m *mockAirbyteService) Create(ctx context.Context, orgID, userID string, req model.CreateConnectorRequest) (*model.ConnectorResponse, error) {
	return m.createFn(ctx, orgID, userID, req)
}
func (m *mockAirbyteService) GetByID(ctx context.Context, orgID, connectorID string) (*model.ConnectorResponse, error) {
	return m.getFn(ctx, orgID, connectorID)
}
func (m *mockAirbyteService) List(ctx context.Context, orgID string, page, pageSize int) (*model.ConnectorListResponse, error) {
	return m.listFn(ctx, orgID, page, pageSize)
}
func (m *mockAirbyteService) Update(ctx context.Context, orgID, connectorID string, req model.UpdateConnectorRequest) (*model.ConnectorResponse, error) {
	return m.updateFn(ctx, orgID, connectorID, req)
}
func (m *mockAirbyteService) Delete(ctx context.Context, orgID, connectorID string) error {
	return m.deleteFn(ctx, orgID, connectorID)
}
func (m *mockAirbyteService) TriggerSync(ctx context.Context, orgID, connectorID string) error {
	return m.triggerFn(ctx, orgID, connectorID)
}
func (m *mockAirbyteService) GetSyncHistory(ctx context.Context, orgID, connectorID string, limit int) ([]model.SyncHistoryResponse, error) {
	return m.historyFn(ctx, orgID, connectorID, limit)
}

func newAirbyteRouter(svc handler.AirbyteServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUserID), "user-1")
		c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
		c.Set(string(middleware.ContextKeyOrgID), "org-abc")
		c.Next()
	})
	h := handler.NewAirbyteHandler(svc)
	const base = "/api/v1/orgs/:org_id/connectors"
	r.POST(base, h.Create)
	r.GET(base, h.List)
	r.GET(base+"/:connector_id", h.Get)
	r.PUT(base+"/:connector_id", h.Update)
	r.DELETE(base+"/:connector_id", h.Delete)
	r.POST(base+"/:connector_id/sync", h.TriggerSync)
	r.GET(base+"/:connector_id/history", h.GetSyncHistory)
	return r
}

func TestCreateConnector_Success(t *testing.T) {
	svc := &mockAirbyteService{
		createFn: func(_ context.Context, orgID, userID string, req model.CreateConnectorRequest) (*model.ConnectorResponse, error) {
			return &model.ConnectorResponse{
				ID:              "conn-1",
				OrgID:           orgID,
				KnowledgeBaseID: req.KnowledgeBaseID,
				Name:            req.Name,
				ConnectorType:   req.ConnectorType,
				SyncMode:        model.SyncModeFullRefresh,
				Status:          model.ConnectorStatusActive,
			}, nil
		},
	}
	r := newAirbyteRouter(svc)
	body, _ := json.Marshal(map[string]string{
		"knowledge_base_id": "kb-1",
		"name":              "My Postgres",
		"connector_type":    "source-postgres",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/connectors", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateConnector_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockAirbyteService{}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	// Missing required fields
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/connectors", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestCreateConnector_ServiceError_Returns400(t *testing.T) {
	svc := &mockAirbyteService{
		createFn: func(_ context.Context, _, _ string, _ model.CreateConnectorRequest) (*model.ConnectorResponse, error) {
			return nil, apierror.NewBadRequest("knowledge base not found or invalid reference")
		},
	}
	r := newAirbyteRouter(svc)
	body, _ := json.Marshal(map[string]string{
		"knowledge_base_id": "kb-bad",
		"name":              "Connector",
		"connector_type":    "source-postgres",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/connectors", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConnector_Success(t *testing.T) {
	svc := &mockAirbyteService{
		getFn: func(_ context.Context, orgID, connectorID string) (*model.ConnectorResponse, error) {
			return &model.ConnectorResponse{
				ID:            connectorID,
				OrgID:         orgID,
				Name:          "Test Connector",
				ConnectorType: "source-postgres",
				SyncMode:      model.SyncModeFullRefresh,
				Status:        model.ConnectorStatusActive,
			}, nil
		},
	}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/connectors/conn-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetConnector_NotFound_Returns404(t *testing.T) {
	svc := &mockAirbyteService{
		getFn: func(_ context.Context, _, _ string) (*model.ConnectorResponse, error) {
			return nil, apierror.NewNotFound("connector not found")
		},
	}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/connectors/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListConnectors_Success(t *testing.T) {
	svc := &mockAirbyteService{
		listFn: func(_ context.Context, _ string, page, pageSize int) (*model.ConnectorListResponse, error) {
			return &model.ConnectorListResponse{
				Data:     []model.ConnectorResponse{},
				Total:    0,
				Page:     page,
				PageSize: pageSize,
			}, nil
		},
	}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/connectors", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp model.ConnectorListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected data to be non-nil empty array")
	}
}

func TestListConnectors_WithPagination(t *testing.T) {
	svc := &mockAirbyteService{
		listFn: func(_ context.Context, _ string, page, pageSize int) (*model.ConnectorListResponse, error) {
			return &model.ConnectorListResponse{
				Data:     []model.ConnectorResponse{},
				Total:    50,
				Page:     page,
				PageSize: pageSize,
			}, nil
		},
	}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/connectors?page=2&page_size=10", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp model.ConnectorListResponse
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

func TestUpdateConnector_Success(t *testing.T) {
	newName := "Updated Connector"
	svc := &mockAirbyteService{
		updateFn: func(_ context.Context, orgID, connectorID string, _ model.UpdateConnectorRequest) (*model.ConnectorResponse, error) {
			return &model.ConnectorResponse{
				ID:            connectorID,
				OrgID:         orgID,
				Name:          newName,
				ConnectorType: "source-postgres",
				SyncMode:      model.SyncModeFullRefresh,
				Status:        model.ConnectorStatusActive,
			}, nil
		},
	}
	r := newAirbyteRouter(svc)
	body, _ := json.Marshal(map[string]string{"name": newName})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/connectors/conn-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateConnector_NotFound_Returns404(t *testing.T) {
	svc := &mockAirbyteService{
		updateFn: func(_ context.Context, _, _ string, _ model.UpdateConnectorRequest) (*model.ConnectorResponse, error) {
			return nil, apierror.NewNotFound("connector not found")
		},
	}
	r := newAirbyteRouter(svc)
	body, _ := json.Marshal(map[string]string{"name": "whatever"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/orgs/org-abc/connectors/bad-id", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDeleteConnector_Success(t *testing.T) {
	svc := &mockAirbyteService{
		deleteFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/connectors/conn-1", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestDeleteConnector_NotFound_Returns404(t *testing.T) {
	svc := &mockAirbyteService{
		deleteFn: func(_ context.Context, _, _ string) error {
			return apierror.NewNotFound("connector not found")
		},
	}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/orgs/org-abc/connectors/bad-id", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTriggerSync_Success(t *testing.T) {
	svc := &mockAirbyteService{
		triggerFn: func(_ context.Context, _, _ string) error { return nil },
	}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/connectors/conn-1/sync", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTriggerSync_ConnectorNotActive_Returns400(t *testing.T) {
	svc := &mockAirbyteService{
		triggerFn: func(_ context.Context, _, _ string) error {
			return apierror.NewBadRequest("connector is not active, current status: paused")
		},
	}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/connectors/conn-1/sync", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSyncHistory_Success(t *testing.T) {
	svc := &mockAirbyteService{
		historyFn: func(_ context.Context, _, _ string, _ int) ([]model.SyncHistoryResponse, error) {
			return []model.SyncHistoryResponse{}, nil
		},
	}
	r := newAirbyteRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/connectors/conn-1/history", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

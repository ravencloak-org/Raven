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

// mockProcessingEventService implements handler.ProcessingEventServicer for unit tests.
type mockProcessingEventService struct {
	transitionFn     func(ctx context.Context, orgID, docID string, toStatus model.ProcessingStatus, errorMsg string) (*model.ProcessingEvent, error)
	listByDocumentFn func(ctx context.Context, orgID, docID string) (*model.ProcessingEventListResponse, error)
}

func (m *mockProcessingEventService) Transition(ctx context.Context, orgID, docID string, toStatus model.ProcessingStatus, errorMsg string) (*model.ProcessingEvent, error) {
	return m.transitionFn(ctx, orgID, docID, toStatus, errorMsg)
}

func (m *mockProcessingEventService) ListByDocumentID(ctx context.Context, orgID, docID string) (*model.ProcessingEventListResponse, error) {
	return m.listByDocumentFn(ctx, orgID, docID)
}

func newProcessingRouter(svc handler.ProcessingEventServicer) *gin.Engine {
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
	h := handler.NewProcessingEventHandler(svc)
	const base = "/api/v1/orgs/:org_id/workspaces/:ws_id/knowledge-bases/:kb_id/documents/:doc_id"
	r.GET(base+"/events", h.ListEvents)
	r.POST(base+"/transitions", h.Transition)
	return r
}

func TestListEvents_Success(t *testing.T) {
	docID := "doc-1"
	fromStatus := model.ProcessingStatusQueued
	svc := &mockProcessingEventService{
		listByDocumentFn: func(_ context.Context, orgID, dID string) (*model.ProcessingEventListResponse, error) {
			return &model.ProcessingEventListResponse{
				Events: []model.ProcessingEvent{
					{
						ID:         "evt-1",
						OrgID:      orgID,
						DocumentID: &dID,
						FromStatus: &fromStatus,
						ToStatus:   model.ProcessingStatusCrawling,
						CreatedAt:  time.Now(),
					},
				},
				Total: 1,
			}, nil
		},
	}
	r := newProcessingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/"+docID+"/events", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp model.ProcessingEventListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(resp.Events))
	}
	if resp.Total != 1 {
		t.Errorf("expected total 1, got %d", resp.Total)
	}
}

func TestListEvents_EmptyReturnsArray(t *testing.T) {
	svc := &mockProcessingEventService{
		listByDocumentFn: func(_ context.Context, _, _ string) (*model.ProcessingEventListResponse, error) {
			return &model.ProcessingEventListResponse{
				Events: []model.ProcessingEvent{},
				Total:  0,
			}, nil
		},
	}
	r := newProcessingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/doc-1/events", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp model.ProcessingEventListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Events == nil {
		t.Error("expected events to be non-nil empty array")
	}
}

func TestListEvents_DocumentNotFound_Returns404(t *testing.T) {
	svc := &mockProcessingEventService{
		listByDocumentFn: func(_ context.Context, _, _ string) (*model.ProcessingEventListResponse, error) {
			return nil, apierror.NewNotFound("document not found")
		},
	}
	r := newProcessingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/bad-id/events", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTransition_Success(t *testing.T) {
	docID := "doc-1"
	fromStatus := model.ProcessingStatusQueued
	svc := &mockProcessingEventService{
		transitionFn: func(_ context.Context, orgID, dID string, toStatus model.ProcessingStatus, errorMsg string) (*model.ProcessingEvent, error) {
			return &model.ProcessingEvent{
				ID:         "evt-1",
				OrgID:      orgID,
				DocumentID: &dID,
				FromStatus: &fromStatus,
				ToStatus:   toStatus,
				CreatedAt:  time.Now(),
			}, nil
		},
	}
	r := newProcessingRouter(svc)
	body, _ := json.Marshal(map[string]string{"to_status": "crawling"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/"+docID+"/transitions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var evt model.ProcessingEvent
	if err := json.Unmarshal(w.Body.Bytes(), &evt); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if evt.ToStatus != model.ProcessingStatusCrawling {
		t.Errorf("expected to_status crawling, got %s", evt.ToStatus)
	}
}

func TestTransition_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockProcessingEventService{}
	r := newProcessingRouter(svc)
	w := httptest.NewRecorder()
	// Missing required to_status field
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/doc-1/transitions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestTransition_InvalidTransition_Returns400(t *testing.T) {
	svc := &mockProcessingEventService{
		transitionFn: func(_ context.Context, _, _ string, _ model.ProcessingStatus, _ string) (*model.ProcessingEvent, error) {
			return nil, apierror.NewBadRequest("invalid status transition from queued to ready")
		},
	}
	r := newProcessingRouter(svc)
	body, _ := json.Marshal(map[string]string{"to_status": "ready"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/doc-1/transitions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTransition_DocumentNotFound_Returns404(t *testing.T) {
	svc := &mockProcessingEventService{
		transitionFn: func(_ context.Context, _, _ string, _ model.ProcessingStatus, _ string) (*model.ProcessingEvent, error) {
			return nil, apierror.NewNotFound("document not found")
		},
	}
	r := newProcessingRouter(svc)
	body, _ := json.Marshal(map[string]string{"to_status": "crawling"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/bad-id/transitions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTransition_WithErrorMessage(t *testing.T) {
	docID := "doc-1"
	fromStatus := model.ProcessingStatusCrawling
	svc := &mockProcessingEventService{
		transitionFn: func(_ context.Context, orgID, dID string, toStatus model.ProcessingStatus, errorMsg string) (*model.ProcessingEvent, error) {
			if errorMsg != "crawl timeout" {
				t.Errorf("expected error message 'crawl timeout', got '%s'", errorMsg)
			}
			return &model.ProcessingEvent{
				ID:           "evt-2",
				OrgID:        orgID,
				DocumentID:   &dID,
				FromStatus:   &fromStatus,
				ToStatus:     toStatus,
				ErrorMessage: errorMsg,
				CreatedAt:    time.Now(),
			}, nil
		},
	}
	r := newProcessingRouter(svc)
	body, _ := json.Marshal(map[string]string{
		"to_status":     "failed",
		"error_message": "crawl timeout",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/"+docID+"/transitions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var evt model.ProcessingEvent
	if err := json.Unmarshal(w.Body.Bytes(), &evt); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if evt.ErrorMessage != "crawl timeout" {
		t.Errorf("expected error_message 'crawl timeout', got '%s'", evt.ErrorMessage)
	}
}

func TestListEvents_ServiceError_Returns500(t *testing.T) {
	svc := &mockProcessingEventService{
		listByDocumentFn: func(_ context.Context, _, _ string) (*model.ProcessingEventListResponse, error) {
			return nil, apierror.NewInternal("database connection lost")
		},
	}
	r := newProcessingRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-abc/workspaces/ws-1/knowledge-bases/kb-1/documents/doc-1/events", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

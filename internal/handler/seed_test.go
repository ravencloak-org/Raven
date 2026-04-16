package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockSeedService implements handler.SeedServicer for unit tests.
type mockSeedService struct {
	seedDemoFn func(ctx context.Context, size string) (*handler.SeedResult, error)
}

func (m *mockSeedService) SeedDemo(ctx context.Context, size string) (*handler.SeedResult, error) {
	if m.seedDemoFn != nil {
		return m.seedDemoFn(ctx, size)
	}
	return &handler.SeedResult{}, nil
}

// newSeedRouter builds a test router for the seed handler.
func newSeedRouter(svc handler.SeedServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewSeedHandler(svc)
	r.POST("/api/v1/admin/seed-demo", h.SeedDemo)
	return r
}

func TestSeedDemo_Success(t *testing.T) {
	expected := &handler.SeedResult{
		OrgID:             "org-123",
		WorkspaceID:       "ws-456",
		KBID:              "kb-789",
		DocumentsEnqueued: 5,
		PipelineStatus:    "running",
	}
	svc := &mockSeedService{
		seedDemoFn: func(_ context.Context, _ string) (*handler.SeedResult, error) {
			return expected, nil
		},
	}

	r := newSeedRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/seed-demo?size=large", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got handler.SeedResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if got.OrgID != expected.OrgID {
		t.Errorf("OrgID = %q, want %q", got.OrgID, expected.OrgID)
	}
	if got.WorkspaceID != expected.WorkspaceID {
		t.Errorf("WorkspaceID = %q, want %q", got.WorkspaceID, expected.WorkspaceID)
	}
	if got.KBID != expected.KBID {
		t.Errorf("KBID = %q, want %q", got.KBID, expected.KBID)
	}
	if got.DocumentsEnqueued != expected.DocumentsEnqueued {
		t.Errorf("DocumentsEnqueued = %d, want %d", got.DocumentsEnqueued, expected.DocumentsEnqueued)
	}
	if got.PipelineStatus != expected.PipelineStatus {
		t.Errorf("PipelineStatus = %q, want %q", got.PipelineStatus, expected.PipelineStatus)
	}
}

func TestSeedDemo_DefaultSizeIsSmall(t *testing.T) {
	var receivedSize string
	svc := &mockSeedService{
		seedDemoFn: func(_ context.Context, size string) (*handler.SeedResult, error) {
			receivedSize = size
			return &handler.SeedResult{
				OrgID:          "org-1",
				WorkspaceID:    "ws-1",
				KBID:           "kb-1",
				PipelineStatus: "running",
			}, nil
		},
	}

	r := newSeedRouter(svc)
	w := httptest.NewRecorder()
	// No ?size= query param.
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/seed-demo", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if receivedSize != "small" {
		t.Errorf("size = %q, want %q", receivedSize, "small")
	}
}

func TestSeedDemo_ServiceError(t *testing.T) {
	svc := &mockSeedService{
		seedDemoFn: func(_ context.Context, _ string) (*handler.SeedResult, error) {
			return nil, apierror.NewInternal("seed failed")
		},
	}

	r := newSeedRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/admin/seed-demo?size=small", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	var errResp apierror.AppError
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}
	if errResp.Code != http.StatusInternalServerError {
		t.Errorf("error code = %d, want %d", errResp.Code, http.StatusInternalServerError)
	}
}

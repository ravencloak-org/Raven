package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// stubAdminSvc implements handler.AdminMarketplaceService with
// programmable behaviour for unit tests. Replaces a real
// *marketplace.AdminModeration so we don't need a Postgres testcontainer
// for handler-layer error-mapping tests.
type stubAdminSvc struct {
	listResult []marketplace.Report
	listErr    error

	approveResult marketplace.ApproveResult
	approveErr    error

	dismissErr error

	gotID uuid.UUID
}

func (s *stubAdminSvc) ListReports(_ context.Context, _ marketplace.ReportStatus, _, _ int) ([]marketplace.Report, error) {
	return s.listResult, s.listErr
}
func (s *stubAdminSvc) Approve(_ context.Context, id uuid.UUID) (marketplace.ApproveResult, error) {
	s.gotID = id
	return s.approveResult, s.approveErr
}
func (s *stubAdminSvc) Dismiss(_ context.Context, id uuid.UUID) error {
	s.gotID = id
	return s.dismissErr
}

// newTestRouter wires the admin handler onto a minimal gin engine.
// Skips auth middleware — the platform-admin gate is unit-tested
// separately in internal/middleware. apierror.ErrorHandler is mounted
// so c.Error -> AppError -> JSON response flows the same as production.
func newTestRouter(t *testing.T, svc handler.AdminMarketplaceService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewAdminMarketplaceHandler(svc)
	r.GET("/admin/marketplace/reports", h.ListReports)
	r.POST("/admin/marketplace/reports/:id/approve", h.Approve)
	r.POST("/admin/marketplace/reports/:id/dismiss", h.Dismiss)
	return r
}

func TestListReports_HappyPath(t *testing.T) {
	t.Parallel()
	rep := marketplace.Report{ID: uuid.New(), ReportedKBID: uuid.New(), Reason: "spam", Status: marketplace.ReportStatusOpen}
	svc := &stubAdminSvc{listResult: []marketplace.Report{rep}}
	r := newTestRouter(t, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,"/admin/marketplace/reports?status=open&limit=10", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	var got struct {
		Reports []marketplace.Report `json:"reports"`
		Limit   int                  `json:"limit"`
		Offset  int                  `json:"offset"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Reports) != 1 || got.Reports[0].ID != rep.ID {
		t.Errorf("unexpected reports payload: %+v", got)
	}
	if got.Limit != 10 || got.Offset != 0 {
		t.Errorf("pagination echo: limit=%d offset=%d", got.Limit, got.Offset)
	}
}

func TestListReports_InvalidStatus(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t, &stubAdminSvc{})

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,"/admin/marketplace/reports?status=bogus", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestListReports_InvalidLimit(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t, &stubAdminSvc{})

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,"/admin/marketplace/reports?limit=abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
}

func TestApprove_HappyPath(t *testing.T) {
	t.Parallel()
	want := marketplace.ApproveResult{
		TakedownID:   uuid.New(),
		TargetKBID:   uuid.New(),
		StrikesAfter: 2,
	}
	svc := &stubAdminSvc{approveResult: want}
	r := newTestRouter(t, svc)

	reportID := uuid.New()
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,"/admin/marketplace/reports/"+reportID.String()+"/approve", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if svc.gotID != reportID {
		t.Errorf("service called with wrong id: got %s, want %s", svc.gotID, reportID)
	}
	var got marketplace.ApproveResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got != want {
		t.Errorf("body: got %+v, want %+v", got, want)
	}
}

func TestApprove_BadUUID(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t, &stubAdminSvc{})

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,"/admin/marketplace/reports/not-a-uuid/approve", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
}

func TestApprove_NotFound(t *testing.T) {
	t.Parallel()
	svc := &stubAdminSvc{approveErr: marketplace.ErrReportNotFound}
	r := newTestRouter(t, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,"/admin/marketplace/reports/"+uuid.NewString()+"/approve", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestApprove_IllegalTransition_409(t *testing.T) {
	t.Parallel()
	svc := &stubAdminSvc{approveErr: errors.Join(marketplace.ErrIllegalTransition, errors.New("from=resolved to=resolved"))}
	r := newTestRouter(t, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,"/admin/marketplace/reports/"+uuid.NewString()+"/approve", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status: want 409, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestDismiss_HappyPath(t *testing.T) {
	t.Parallel()
	svc := &stubAdminSvc{}
	r := newTestRouter(t, svc)

	reportID := uuid.New()
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,"/admin/marketplace/reports/"+reportID.String()+"/dismiss", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", w.Code)
	}
	if svc.gotID != reportID {
		t.Errorf("service called with wrong id: got %s", svc.gotID)
	}
	if !strings.Contains(w.Body.String(), `"status":"dismissed"`) {
		t.Errorf("expected dismissed echo, got body=%s", w.Body.String())
	}
}

func TestDismiss_NotFound(t *testing.T) {
	t.Parallel()
	svc := &stubAdminSvc{dismissErr: marketplace.ErrReportNotFound}
	r := newTestRouter(t, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,"/admin/marketplace/reports/"+uuid.NewString()+"/dismiss", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestDismiss_IllegalTransition_409(t *testing.T) {
	t.Parallel()
	svc := &stubAdminSvc{dismissErr: marketplace.ErrIllegalTransition}
	r := newTestRouter(t, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,"/admin/marketplace/reports/"+uuid.NewString()+"/dismiss", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status: want 409, got %d (body=%s)", w.Code, w.Body.String())
	}
}

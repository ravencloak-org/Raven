package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/marketplace"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// stubDMCASvc is a hand-rolled stub for AdminDMCAService so the handler
// unit tests do not need a Postgres testcontainer. Mirrors stubAdminSvc
// in admin_marketplace_test.go.
type stubDMCASvc struct {
	submitResult marketplace.DMCANotice
	submitErr    error

	counterErr error

	listResult []marketplace.DMCANotice
	listErr    error

	gotIn            marketplace.DMCANoticeInput
	gotNoticeID      uuid.UUID
	gotCounterText   string
	gotListStatus    marketplace.DMCAStatus
	gotListLimit     int
	gotListOffset    int
	gotListCalled    bool
	gotSubmitCalled  bool
	gotCounterCalled bool
}

func (s *stubDMCASvc) Submit(_ context.Context, in marketplace.DMCANoticeInput) (marketplace.DMCANotice, error) {
	s.gotSubmitCalled = true
	s.gotIn = in
	return s.submitResult, s.submitErr
}
func (s *stubDMCASvc) SubmitCounterNotice(_ context.Context, id uuid.UUID, text string) error {
	s.gotCounterCalled = true
	s.gotNoticeID = id
	s.gotCounterText = text
	return s.counterErr
}
func (s *stubDMCASvc) ListNotices(_ context.Context, status marketplace.DMCAStatus, limit, offset int) ([]marketplace.DMCANotice, error) {
	s.gotListCalled = true
	s.gotListStatus = status
	s.gotListLimit = limit
	s.gotListOffset = offset
	return s.listResult, s.listErr
}

// newDMCARouter wires the admin DMCA handler on a minimal gin engine.
// Skips auth — the platform-admin gate is unit-tested separately.
func newDMCARouter(t *testing.T, svc handler.AdminDMCAService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewAdminDMCAHandler(svc)
	r.GET("/admin/marketplace/dmca", h.List)
	r.POST("/admin/marketplace/dmca", h.Submit)
	r.POST("/admin/marketplace/dmca/:id/counter-notice", h.SubmitCounterNotice)
	return r
}

// validSubmitBody returns a JSON body that passes inline validation.
func validSubmitBody(t *testing.T, kbID uuid.UUID) *bytes.Buffer {
	t.Helper()
	body := map[string]string{
		"target_kb_id":   kbID.String(),
		"notice_text":    "Material at /marketplace/foo/bar infringes my copyright. — signed.",
		"claimant_email": "rights@example.com",
		"claimant_name":  "Acme Rights Holder",
	}
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(buf)
}

// ── Submit ──────────────────────────────────────────────────────────────────

func TestSubmit_HappyPath(t *testing.T) {
	t.Parallel()
	kbID := uuid.New()
	noticeID := uuid.New()
	svc := &stubDMCASvc{
		submitResult: marketplace.DMCANotice{
			ID:                      noticeID,
			TargetKBID:              kbID,
			NoticeText:              "ok",
			ClaimantEmail:           "rights@example.com",
			ClaimantName:            "Acme",
			CounterNoticeWindowEnds: time.Now().Add(14 * 24 * time.Hour),
			Status:                  marketplace.DMCAStatusPending,
		},
	}
	r := newDMCARouter(t, svc)

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca", validSubmitBody(t, kbID))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: want 201, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !svc.gotSubmitCalled {
		t.Fatal("svc.Submit was not called")
	}
	if svc.gotIn.TargetKBID != kbID {
		t.Errorf("Submit input target_kb_id: want %s, got %s", kbID, svc.gotIn.TargetKBID)
	}
	var got marketplace.DMCANotice
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != noticeID {
		t.Errorf("response id: want %s, got %s", noticeID, got.ID)
	}
}

func TestSubmit_BadJSON(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca", strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
	if svc.gotSubmitCalled {
		t.Error("svc.Submit should not be called on bad JSON")
	}
}

func TestSubmit_BadTargetUUID(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	body := strings.NewReader(`{"target_kb_id":"not-a-uuid","notice_text":"x","claimant_email":"a@b","claimant_name":"a"}`)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
}

func TestSubmit_NotFound(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{submitErr: marketplace.ErrDMCATargetKBNotFound}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca", validSubmitBody(t, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestSubmit_AlreadyPending409(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{submitErr: errors.Join(marketplace.ErrDMCAAlreadyPending, errors.New("inner"))}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca", validSubmitBody(t, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("status: want 409, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestSubmit_InvalidInput400(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{submitErr: marketplace.ErrDMCAInvalidInput}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca", validSubmitBody(t, uuid.New()))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
}

// ── SubmitCounterNotice ────────────────────────────────────────────────────

func TestCounterNotice_HappyPath(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{}
	r := newDMCARouter(t, svc)
	id := uuid.New()
	body := strings.NewReader(`{"counter_notice_text":"the takedown was a mistake — signed."}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca/"+id.String()+"/counter-notice", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !svc.gotCounterCalled {
		t.Fatal("svc.SubmitCounterNotice was not called")
	}
	if svc.gotNoticeID != id {
		t.Errorf("notice id: want %s, got %s", id, svc.gotNoticeID)
	}
	if !strings.Contains(w.Body.String(), `"status":"counter_filed"`) {
		t.Errorf("expected counter_filed echo, got %s", w.Body.String())
	}
}

func TestCounterNotice_BadUUID(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca/not-a-uuid/counter-notice", strings.NewReader(`{"counter_notice_text":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
	if svc.gotCounterCalled {
		t.Error("service should not be called on bad UUID")
	}
}

func TestCounterNotice_NotFound(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{counterErr: marketplace.ErrDMCANoticeNotFound}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca/"+uuid.NewString()+"/counter-notice", strings.NewReader(`{"counter_notice_text":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", w.Code)
	}
}

func TestCounterNotice_IllegalTransition409(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{counterErr: marketplace.ErrDMCAIllegalTransition}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/marketplace/dmca/"+uuid.NewString()+"/counter-notice", strings.NewReader(`{"counter_notice_text":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("status: want 409, got %d", w.Code)
	}
}

// ── List ──────────────────────────────────────────────────────────────────

func TestList_HappyPath(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{listResult: []marketplace.DMCANotice{
		{ID: uuid.New(), Status: marketplace.DMCAStatusPending},
	}}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/marketplace/dmca?status=pending&limit=10", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if svc.gotListStatus != marketplace.DMCAStatusPending {
		t.Errorf("list status: want pending, got %q", svc.gotListStatus)
	}
	if svc.gotListLimit != 10 {
		t.Errorf("list limit: want 10, got %d", svc.gotListLimit)
	}
}

func TestList_InvalidStatus400(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/marketplace/dmca?status=bogus", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", w.Code)
	}
	if svc.gotListCalled {
		t.Error("service should not be called on bad status filter")
	}
}

func TestList_EmptyStatusOK(t *testing.T) {
	t.Parallel()
	svc := &stubDMCASvc{listResult: []marketplace.DMCANotice{}}
	r := newDMCARouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/marketplace/dmca", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if svc.gotListStatus != "" {
		t.Errorf("empty status filter should pass through as empty string, got %q", svc.gotListStatus)
	}
}

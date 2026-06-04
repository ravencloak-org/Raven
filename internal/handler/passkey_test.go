package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/service"
)

// --- fakes -----------------------------------------------------------------

// fakePasskeyService is a hand-written stub of the full handler-facing
// interface — core list/delete plus label list/update/delete. Tests
// pre-load credentials and label rows, optionally prime errors, and
// inspect the captured-call slices to assert on writes.
type fakePasskeyService struct {
	// Core side.
	credentials []service.PasskeyCredential
	listErr     error
	deleteErr   error
	deletedID   string

	// Label side.
	rows           map[string]service.PasskeyLabel
	listLabelsErr  error
	updateErr      error
	deleteLabelErr error
	updateCalls    []labelUpdateCall
	deleteCalls    []labelDeleteCall
}

type labelUpdateCall struct{ userID, credentialID, label string }
type labelDeleteCall struct{ userID, credentialID string }

func (f *fakePasskeyService) ListByUser(_ context.Context, _ string) ([]service.PasskeyCredential, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]service.PasskeyCredential, len(f.credentials))
	copy(out, f.credentials)
	return out, nil
}

func (f *fakePasskeyService) DeleteCredential(_ context.Context, _, credentialID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedID = credentialID
	return nil
}

func (f *fakePasskeyService) ListLabelsForUser(_ context.Context, _ string) (map[string]service.PasskeyLabel, error) {
	if f.listLabelsErr != nil {
		return nil, f.listLabelsErr
	}
	out := make(map[string]service.PasskeyLabel, len(f.rows))
	for k, v := range f.rows {
		out[k] = v
	}
	return out, nil
}

func (f *fakePasskeyService) UpdateLabel(_ context.Context, userID, credentialID, label string) error {
	f.updateCalls = append(f.updateCalls, labelUpdateCall{userID, credentialID, label})
	if f.updateErr != nil {
		return f.updateErr
	}
	// Mirror real repository semantics: 0 rows ⇒ ErrPasskeyLabelNotFound.
	// Tests that want the "happy path" pre-load f.rows with the credential.
	row, ok := f.rows[credentialID]
	if !ok {
		return service.ErrPasskeyLabelNotFound
	}
	row.Label = label
	f.rows[credentialID] = row
	return nil
}

func (f *fakePasskeyService) DeleteLabel(_ context.Context, userID, credentialID string) error {
	f.deleteCalls = append(f.deleteCalls, labelDeleteCall{userID, credentialID})
	return f.deleteLabelErr
}

// newPasskeyCtx constructs a gin context with the session keys populated so
// requireSession() lets the handler through. The returned recorder is the
// raw httptest.ResponseRecorder; we capture the post-handler status by
// reading c.Writer.Status() rather than rec.Code because gin's
// CreateTestContext path defers WriteHeader until a body is written.
func newPasskeyCtx(method, target, credentialIDParam, bodyJSON string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	var body io.Reader
	if bodyJSON != "" {
		body = bytes.NewBufferString(bodyJSON)
	}
	c.Request = httptest.NewRequestWithContext(context.Background(), method, target, body)
	if bodyJSON != "" {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	c.Set(string(middleware.ContextKeyUserID), "user-uuid-1")
	c.Set(string(middleware.ContextKeyExternalID), "st-user-1")
	if credentialIDParam != "" {
		c.Params = gin.Params{{Key: "credential_id", Value: credentialIDParam}}
	}
	return c, rec
}

// gotStatus returns the gin writer's recorded status code, falling back to
// the underlying recorder for paths that wrote a body via gin's render layer.
func gotStatus(c *gin.Context, rec *httptest.ResponseRecorder) int {
	if s := c.Writer.Status(); s != 0 {
		return s
	}
	return rec.Code
}

// --- List ------------------------------------------------------------------

func TestPasskeyHandler_List(t *testing.T) {
	createdAt := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	lastUsed := time.Date(2026, 6, 2, 11, 0, 0, 0, time.UTC)
	labelCreated := time.Date(2026, 6, 1, 10, 0, 5, 0, time.UTC)

	tests := []struct {
		name        string
		creds       []service.PasskeyCredential
		labels      map[string]service.PasskeyLabel
		wantStatus  int
		wantBodyHas []string
	}{
		{
			name: "merged credentials and labels",
			creds: []service.PasskeyCredential{
				{CredentialID: "cred-1", CreatedAt: createdAt, LastUsedAt: &lastUsed},
				{CredentialID: "cred-2", CreatedAt: createdAt},
			},
			labels: map[string]service.PasskeyLabel{
				"cred-1": {CredentialID: "cred-1", Label: "MacBook Pro Touch ID", CreatedAt: labelCreated, LastUsedAt: &lastUsed},
				"cred-2": {CredentialID: "cred-2", Label: "iPhone Face ID", CreatedAt: labelCreated},
			},
			wantStatus:  http.StatusOK,
			wantBodyHas: []string{`"credential_id":"cred-1"`, `"label":"MacBook Pro Touch ID"`, `"label":"iPhone Face ID"`},
		},
		{
			name: "empty label rows still surfaces credentials with fallback label",
			creds: []service.PasskeyCredential{
				{CredentialID: "cred-orphan", CreatedAt: createdAt},
			},
			labels:      map[string]service.PasskeyLabel{},
			wantStatus:  http.StatusOK,
			wantBodyHas: []string{`"credential_id":"cred-orphan"`, `"label":"Passkey"`},
		},
		{
			name:        "empty everything returns empty array, not null",
			creds:       []service.PasskeyCredential{},
			labels:      map[string]service.PasskeyLabel{},
			wantStatus:  http.StatusOK,
			wantBodyHas: []string{`[]`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakePasskeyService{credentials: tt.creds, rows: tt.labels}
			h := NewPasskeyHandler(svc)

			c, rec := newPasskeyCtx(http.MethodGet, "/api/v1/me/passkeys", "", "")
			h.List(c)

			if got := gotStatus(c, rec); got != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s)", got, tt.wantStatus, rec.Body.String())
			}
			for _, frag := range tt.wantBodyHas {
				if !strings.Contains(rec.Body.String(), frag) {
					t.Errorf("body missing %q\ngot: %s", frag, rec.Body.String())
				}
			}
		})
	}
}

func TestPasskeyHandler_List_CoreUnavailable(t *testing.T) {
	svc := &fakePasskeyService{listErr: errors.New("connection refused")}
	h := NewPasskeyHandler(svc)

	c, rec := newPasskeyCtx(http.MethodGet, "/api/v1/me/passkeys", "", "")
	// Wire the gin error chain so the handler's _ = c.Error(...) gets surfaced
	// with the right status code. Without the apierror.ErrorHandler middleware
	// gin records the error but leaves the status at 200; we explicitly check
	// the recorded c.Errors list instead.
	h.List(c)

	if len(c.Errors) == 0 {
		t.Fatalf("expected an error to be set; got none. status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(c.Errors.Last().Error(), "passkey list failed") {
		t.Errorf("unexpected error: %v", c.Errors.Last())
	}
}

// --- Patch -----------------------------------------------------------------

func TestPasskeyHandler_Patch(t *testing.T) {
	// New ownership model: PATCH only succeeds when a label row already
	// exists for (credential_id, user_id). The registration hook is
	// responsible for inserting the initial row; the user-facing PATCH
	// only renames it.
	tests := []struct {
		name           string
		body           string
		existingLabels map[string]service.PasskeyLabel
		wantStatus     int
		wantUpdate     bool
		wantLabel      string
	}{
		{
			name:           "rename existing label",
			body:           `{"label":"Renamed"}`,
			existingLabels: map[string]service.PasskeyLabel{"cred-1": {CredentialID: "cred-1", Label: "Old"}},
			wantStatus:     http.StatusNoContent,
			wantUpdate:     true,
			wantLabel:      "Renamed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakePasskeyService{rows: tt.existingLabels}
			h := NewPasskeyHandler(svc)

			c, rec := newPasskeyCtx(http.MethodPatch, "/api/v1/me/passkeys/cred-1", "cred-1", tt.body)
			h.Patch(c)

			if got := gotStatus(c, rec); got != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s, errs=%v)", got, tt.wantStatus, rec.Body.String(), c.Errors)
			}
			if tt.wantUpdate {
				if len(svc.updateCalls) != 1 {
					t.Fatalf("expected 1 update call, got %d", len(svc.updateCalls))
				}
				call := svc.updateCalls[0]
				if call.label != tt.wantLabel {
					t.Errorf("update label = %q, want %q", call.label, tt.wantLabel)
				}
				if call.credentialID != "cred-1" {
					t.Errorf("update credentialID = %q, want cred-1", call.credentialID)
				}
				if call.userID != "user-uuid-1" {
					t.Errorf("update userID = %q, want user-uuid-1", call.userID)
				}
			}
			// PATCH must NOT call into the SuperTokens core — that is the
			// whole point of the ownsCredential removal.
			if svc.deletedID != "" {
				t.Errorf("PATCH must not trigger any core write; got delete for %q", svc.deletedID)
			}
		})
	}
}

func TestPasskeyHandler_Patch_UnknownCredentialReturns404(t *testing.T) {
	// The label store is empty, so the atomic UPDATE matches 0 rows. The
	// handler must surface that as 404 (and never call into the core).
	svc := &fakePasskeyService{rows: map[string]service.PasskeyLabel{}}
	h := NewPasskeyHandler(svc)

	c, _ := newPasskeyCtx(http.MethodPatch, "/api/v1/me/passkeys/cred-evil", "cred-evil", `{"label":"hax"}`)
	h.Patch(c)

	if len(c.Errors) == 0 {
		t.Fatalf("expected a 404 error; got none")
	}
	if !strings.Contains(c.Errors.Last().Error(), "passkey not found") {
		t.Errorf("unexpected error: %v", c.Errors.Last())
	}
	if len(svc.updateCalls) != 1 {
		t.Errorf("expected exactly one UpdateLabel attempt, got %d", len(svc.updateCalls))
	}
}

func TestPasskeyHandler_Patch_BadBody(t *testing.T) {
	svc := &fakePasskeyService{rows: map[string]service.PasskeyLabel{"cred-1": {CredentialID: "cred-1"}}}
	h := NewPasskeyHandler(svc)

	c, _ := newPasskeyCtx(http.MethodPatch, "/api/v1/me/passkeys/cred-1", "cred-1", `{}`)
	h.Patch(c)

	if len(c.Errors) == 0 {
		t.Fatalf("expected validation error; got none")
	}
	if len(svc.updateCalls) != 0 {
		t.Errorf("bad body must not reach the service; got %d update calls", len(svc.updateCalls))
	}
}

// --- Delete ---------------------------------------------------------------

func TestPasskeyHandler_Delete(t *testing.T) {
	svc := &fakePasskeyService{
		credentials: []service.PasskeyCredential{{CredentialID: "cred-1"}},
		rows:        map[string]service.PasskeyLabel{"cred-1": {CredentialID: "cred-1", Label: "Mac"}},
	}
	h := NewPasskeyHandler(svc)

	c, rec := newPasskeyCtx(http.MethodDelete, "/api/v1/me/passkeys/cred-1", "cred-1", "")
	h.Delete(c)

	if got := gotStatus(c, rec); got != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s, errs=%v)", got, rec.Body.String(), c.Errors)
	}
	if svc.deletedID != "cred-1" {
		t.Errorf("svc.Delete saw credentialID %q, want cred-1", svc.deletedID)
	}
	if len(svc.deleteCalls) != 1 || svc.deleteCalls[0].credentialID != "cred-1" {
		t.Errorf("expected one label-store delete for cred-1; got %v", svc.deleteCalls)
	}
}

func TestPasskeyHandler_Delete_CoreReturnsNotFound(t *testing.T) {
	// Ownership is now enforced by the core's own 404. The handler must
	// surface that as 404 and never touch the label row.
	svc := &fakePasskeyService{
		credentials: []service.PasskeyCredential{{CredentialID: "cred-1"}},
		deleteErr:   service.ErrPasskeyNotFound,
	}
	h := NewPasskeyHandler(svc)

	c, _ := newPasskeyCtx(http.MethodDelete, "/api/v1/me/passkeys/cred-1", "cred-1", "")
	h.Delete(c)

	if len(c.Errors) == 0 {
		t.Fatalf("expected 404; got none")
	}
	if !strings.Contains(c.Errors.Last().Error(), "passkey not found") {
		t.Errorf("unexpected error: %v", c.Errors.Last())
	}
	if len(svc.deleteCalls) != 0 {
		t.Errorf("core-not-found delete must not touch label store; got %d calls", len(svc.deleteCalls))
	}
}

// --- PasskeyService HTTP-level tests --------------------------------------

// TestPasskeyService_ListByUser hits a real net/http test server so we
// exercise the URL composition, headers and JSON decoding.
func TestPasskeyService_ListByUser(t *testing.T) {
	createdMs := int64(1748774400000) // 2026-06-01T10:00:00Z
	lastMs := int64(1748860800000)    // 2026-06-02T10:00:00Z

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasPrefix(r.URL.Path, "/recipe/webauthn/user/credentials") {
			http.Error(w, "wrong path: "+r.URL.Path, http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("userId") != "st-user-1" {
			http.Error(w, "missing userId", http.StatusBadRequest)
			return
		}
		if r.Header.Get("cdi-version") == "" {
			http.Error(w, "missing cdi-version", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "OK",
			"credentials": []map[string]interface{}{
				{"credentialId": "cred-1", "createdAt": createdMs, "lastUsedAt": lastMs},
				{"credentialId": "cred-2", "createdAt": createdMs},
			},
		})
	}))
	defer srv.Close()

	svc := service.NewPasskeyService(srv.URL, "test-api-key", nil, nil)
	got, err := svc.ListByUser(context.Background(), "st-user-1")
	if err != nil {
		t.Fatalf("ListByUser err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d creds, want 2", len(got))
	}
	if got[0].CredentialID != "cred-1" {
		t.Errorf("cred[0].CredentialID = %q, want cred-1", got[0].CredentialID)
	}
	if got[0].LastUsedAt == nil {
		t.Errorf("cred[0].LastUsedAt = nil, want non-nil")
	}
	if got[1].LastUsedAt != nil {
		t.Errorf("cred[1].LastUsedAt should be nil")
	}
}

func TestPasskeyService_DeleteCredential_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"CREDENTIAL_NOT_FOUND_ERROR"}`))
	}))
	defer srv.Close()

	svc := service.NewPasskeyService(srv.URL, "", nil, nil)
	err := svc.DeleteCredential(context.Background(), "st-user-1", "cred-missing")
	if !errors.Is(err, service.ErrPasskeyNotFound) {
		t.Fatalf("err = %v, want ErrPasskeyNotFound", err)
	}
}

func TestPasskeyService_DeleteCredential_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	}))
	defer srv.Close()

	svc := service.NewPasskeyService(srv.URL, "", nil, nil)
	if err := svc.DeleteCredential(context.Background(), "st-user-1", "cred-1"); err != nil {
		t.Fatalf("DeleteCredential err: %v", err)
	}
}

func TestPasskeyService_ListByUser_CoreError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := service.NewPasskeyService(srv.URL, "", nil, nil)
	_, err := svc.ListByUser(context.Background(), "st-user-1")
	if err == nil {
		t.Fatalf("expected error from 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("unexpected error: %v", err)
	}
}

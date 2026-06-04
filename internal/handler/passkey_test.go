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

// fakePasskeySvc is a hand-written stub. Each test pre-loads the credentials
// the SuperTokens core would have returned and (optionally) primes errors on
// list / delete.
type fakePasskeySvc struct {
	credentials []service.PasskeyCredential
	listErr     error
	deleteErr   error
	deletedID   string
}

func (f *fakePasskeySvc) ListByUser(_ context.Context, _ string) ([]service.PasskeyCredential, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]service.PasskeyCredential, len(f.credentials))
	copy(out, f.credentials)
	return out, nil
}

func (f *fakePasskeySvc) DeleteCredential(_ context.Context, _, credentialID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedID = credentialID
	return nil
}

// fakeLabelStore captures every call so tests can assert on writes without
// needing a live Postgres. ListRows is keyed by credential_id.
type fakeLabelStore struct {
	rows         map[string]passkeyLabelRow
	upsertCalls  []labelUpsertCall
	deleteCalls  []labelDeleteCall
	listErr      error
	upsertErr    error
	deleteLabErr error
}

type labelUpsertCall struct{ userID, credentialID, label string }
type labelDeleteCall struct{ userID, credentialID string }

func (f *fakeLabelStore) ListLabelsForUser(_ context.Context, _ string) (map[string]passkeyLabelRow, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make(map[string]passkeyLabelRow, len(f.rows))
	for k, v := range f.rows {
		out[k] = v
	}
	return out, nil
}

func (f *fakeLabelStore) UpsertLabel(_ context.Context, userID, credentialID, label string) error {
	f.upsertCalls = append(f.upsertCalls, labelUpsertCall{userID, credentialID, label})
	return f.upsertErr
}

func (f *fakeLabelStore) DeleteLabel(_ context.Context, userID, credentialID string) error {
	f.deleteCalls = append(f.deleteCalls, labelDeleteCall{userID, credentialID})
	return f.deleteLabErr
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
		labels      map[string]passkeyLabelRow
		wantStatus  int
		wantBodyHas []string
	}{
		{
			name: "merged credentials and labels",
			creds: []service.PasskeyCredential{
				{CredentialID: "cred-1", CreatedAt: createdAt, LastUsedAt: &lastUsed},
				{CredentialID: "cred-2", CreatedAt: createdAt},
			},
			labels: map[string]passkeyLabelRow{
				"cred-1": {Label: "MacBook Pro Touch ID", CreatedAt: labelCreated, LastUsedAt: &lastUsed},
				"cred-2": {Label: "iPhone Face ID", CreatedAt: labelCreated},
			},
			wantStatus:  http.StatusOK,
			wantBodyHas: []string{`"credential_id":"cred-1"`, `"label":"MacBook Pro Touch ID"`, `"label":"iPhone Face ID"`},
		},
		{
			name: "empty label rows still surfaces credentials with fallback label",
			creds: []service.PasskeyCredential{
				{CredentialID: "cred-orphan", CreatedAt: createdAt},
			},
			labels:      map[string]passkeyLabelRow{}, // no label rows yet
			wantStatus:  http.StatusOK,
			wantBodyHas: []string{`"credential_id":"cred-orphan"`, `"label":"Passkey"`},
		},
		{
			name:        "empty everything returns empty array, not null",
			creds:       []service.PasskeyCredential{},
			labels:      map[string]passkeyLabelRow{},
			wantStatus:  http.StatusOK,
			wantBodyHas: []string{`[]`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakePasskeySvc{credentials: tt.creds}
			store := &fakeLabelStore{rows: tt.labels}
			h := NewPasskeyHandlerWithStore(svc, store)

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
	svc := &fakePasskeySvc{listErr: errors.New("connection refused")}
	store := &fakeLabelStore{rows: map[string]passkeyLabelRow{}}
	h := NewPasskeyHandlerWithStore(svc, store)

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
	tests := []struct {
		name           string
		body           string
		existingCreds  []service.PasskeyCredential
		existingLabels map[string]passkeyLabelRow
		wantStatus     int
		wantUpsert     bool
		wantLabel      string
	}{
		{
			name:          "insert when no row exists",
			body:          `{"label":"Yubikey 5C"}`,
			existingCreds: []service.PasskeyCredential{{CredentialID: "cred-1"}},
			wantStatus:    http.StatusNoContent,
			wantUpsert:    true,
			wantLabel:     "Yubikey 5C",
		},
		{
			name:           "update when row already exists",
			body:           `{"label":"Renamed"}`,
			existingCreds:  []service.PasskeyCredential{{CredentialID: "cred-1"}},
			existingLabels: map[string]passkeyLabelRow{"cred-1": {Label: "Old"}},
			wantStatus:     http.StatusNoContent,
			wantUpsert:     true,
			wantLabel:      "Renamed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakePasskeySvc{credentials: tt.existingCreds}
			store := &fakeLabelStore{rows: tt.existingLabels}
			h := NewPasskeyHandlerWithStore(svc, store)

			c, rec := newPasskeyCtx(http.MethodPatch, "/api/v1/me/passkeys/cred-1", "cred-1", tt.body)
			h.Patch(c)

			if got := gotStatus(c, rec); got != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body=%s, errs=%v)", got, tt.wantStatus, rec.Body.String(), c.Errors)
			}
			if tt.wantUpsert {
				if len(store.upsertCalls) != 1 {
					t.Fatalf("expected 1 upsert call, got %d", len(store.upsertCalls))
				}
				call := store.upsertCalls[0]
				if call.label != tt.wantLabel {
					t.Errorf("upsert label = %q, want %q", call.label, tt.wantLabel)
				}
				if call.credentialID != "cred-1" {
					t.Errorf("upsert credentialID = %q, want cred-1", call.credentialID)
				}
				if call.userID != "user-uuid-1" {
					t.Errorf("upsert userID = %q, want user-uuid-1", call.userID)
				}
			}
		})
	}
}

func TestPasskeyHandler_Patch_OwnershipForbidden(t *testing.T) {
	// The core returns a different user's credentials — patching cred-evil
	// must 403 and never call the label store.
	svc := &fakePasskeySvc{credentials: []service.PasskeyCredential{{CredentialID: "cred-mine"}}}
	store := &fakeLabelStore{}
	h := NewPasskeyHandlerWithStore(svc, store)

	c, _ := newPasskeyCtx(http.MethodPatch, "/api/v1/me/passkeys/cred-evil", "cred-evil", `{"label":"hax"}`)
	h.Patch(c)

	if len(c.Errors) == 0 {
		t.Fatalf("expected an error; got none")
	}
	if !strings.Contains(c.Errors.Last().Error(), "credential does not belong") {
		t.Errorf("unexpected error: %v", c.Errors.Last())
	}
	if len(store.upsertCalls) != 0 {
		t.Errorf("ownership-failed patch must not write a label; got %d calls", len(store.upsertCalls))
	}
}

func TestPasskeyHandler_Patch_BadBody(t *testing.T) {
	svc := &fakePasskeySvc{credentials: []service.PasskeyCredential{{CredentialID: "cred-1"}}}
	store := &fakeLabelStore{}
	h := NewPasskeyHandlerWithStore(svc, store)

	c, _ := newPasskeyCtx(http.MethodPatch, "/api/v1/me/passkeys/cred-1", "cred-1", `{}`)
	h.Patch(c)

	if len(c.Errors) == 0 {
		t.Fatalf("expected validation error; got none")
	}
}

// --- Delete ---------------------------------------------------------------

func TestPasskeyHandler_Delete(t *testing.T) {
	svc := &fakePasskeySvc{credentials: []service.PasskeyCredential{{CredentialID: "cred-1"}}}
	store := &fakeLabelStore{rows: map[string]passkeyLabelRow{"cred-1": {Label: "Mac"}}}
	h := NewPasskeyHandlerWithStore(svc, store)

	c, rec := newPasskeyCtx(http.MethodDelete, "/api/v1/me/passkeys/cred-1", "cred-1", "")
	h.Delete(c)

	if got := gotStatus(c, rec); got != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (body=%s, errs=%v)", got, rec.Body.String(), c.Errors)
	}
	if svc.deletedID != "cred-1" {
		t.Errorf("svc.Delete saw credentialID %q, want cred-1", svc.deletedID)
	}
	if len(store.deleteCalls) != 1 || store.deleteCalls[0].credentialID != "cred-1" {
		t.Errorf("expected one label-store delete for cred-1; got %v", store.deleteCalls)
	}
}

func TestPasskeyHandler_Delete_OwnershipForbidden(t *testing.T) {
	svc := &fakePasskeySvc{credentials: []service.PasskeyCredential{{CredentialID: "someone-elses"}}}
	store := &fakeLabelStore{}
	h := NewPasskeyHandlerWithStore(svc, store)

	c, _ := newPasskeyCtx(http.MethodDelete, "/api/v1/me/passkeys/cred-1", "cred-1", "")
	h.Delete(c)

	if len(c.Errors) == 0 {
		t.Fatalf("expected 403 error; got none")
	}
	if svc.deletedID != "" {
		t.Errorf("ownership-failed delete must not call core; got delete for %q", svc.deletedID)
	}
}

func TestPasskeyHandler_Delete_CoreReturnsNotFound(t *testing.T) {
	svc := &fakePasskeySvc{
		credentials: []service.PasskeyCredential{{CredentialID: "cred-1"}},
		deleteErr:   service.ErrPasskeyNotFound,
	}
	store := &fakeLabelStore{}
	h := NewPasskeyHandlerWithStore(svc, store)

	c, _ := newPasskeyCtx(http.MethodDelete, "/api/v1/me/passkeys/cred-1", "cred-1", "")
	h.Delete(c)

	if len(c.Errors) == 0 {
		t.Fatalf("expected 404; got none")
	}
	if !strings.Contains(c.Errors.Last().Error(), "passkey not found") {
		t.Errorf("unexpected error: %v", c.Errors.Last())
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

	svc := service.NewPasskeyService(srv.URL, "test-api-key", nil)
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

	svc := service.NewPasskeyService(srv.URL, "", nil)
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

	svc := service.NewPasskeyService(srv.URL, "", nil)
	if err := svc.DeleteCredential(context.Background(), "st-user-1", "cred-1"); err != nil {
		t.Fatalf("DeleteCredential err: %v", err)
	}
}

func TestPasskeyService_ListByUser_CoreError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := service.NewPasskeyService(srv.URL, "", nil)
	_, err := svc.ListByUser(context.Background(), "st-user-1")
	if err == nil {
		t.Fatalf("expected error from 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("unexpected error: %v", err)
	}
}

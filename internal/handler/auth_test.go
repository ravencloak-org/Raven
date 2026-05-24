package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

type stubAuthSvc struct {
	getByExternalIDErr error
	createdEmail       string
	createdName        string
	createdUser        *model.User
}

func (s *stubAuthSvc) GetByExternalID(_ context.Context, _ string) (*model.User, error) {
	return nil, s.getByExternalIDErr
}

func (s *stubAuthSvc) Create(_ context.Context, _, email, name string) (*model.User, error) {
	s.createdEmail = email
	s.createdName = name
	return s.createdUser, nil
}

type stubEmailLookup struct {
	returnEmail string
	err         error
	calledWith  string
}

func (s *stubEmailLookup) LookupEmail(_ context.Context, externalID string) (string, error) {
	s.calledWith = externalID
	return s.returnEmail, s.err
}

func newCallbackCtx(externalID, email, name string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/auth/callback", nil)
	c.Set(string(middleware.ContextKeyExternalID), externalID)
	c.Set(string(middleware.ContextKeyEmail), email)
	c.Set(string(middleware.ContextKeyUserName), name)
	return c, rec
}

// On Google ThirdParty signin the SuperTokens access-token payload doesn't
// carry an email, so the handler-side EmailLookup fallback is what actually
// populates users.email on first login. This test pins that wiring: when the
// middleware-set email is empty and a new user is being created, the handler
// must call EmailLookup and pass the resolved value through to svc.Create.
func TestCallback_FallsBackToEmailLookupOnNewUserCreate(t *testing.T) {
	svc := &stubAuthSvc{
		getByExternalIDErr: apierror.NewNotFound("user not found"),
		createdUser:        &model.User{ID: "user-1"},
	}
	lookup := &stubEmailLookup{returnEmail: "jobin@example.com"}
	h := NewAuthHandler(svc, lookup)

	c, rec := newCallbackCtx("st-user-abc", "", "Jobin Lawrance")
	h.Callback(c)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if lookup.calledWith != "st-user-abc" {
		t.Errorf("EmailLookup called with %q, want %q", lookup.calledWith, "st-user-abc")
	}
	if svc.createdEmail != "jobin@example.com" {
		t.Errorf("svc.Create got email %q, want %q", svc.createdEmail, "jobin@example.com")
	}
	if svc.createdName != "Jobin Lawrance" {
		t.Errorf("svc.Create got name %q, want %q", svc.createdName, "Jobin Lawrance")
	}
}

// When the session claims already carry an email (e.g. SuperTokens custom
// override is in use) we must NOT pay for the lookup round-trip.
func TestCallback_SkipsLookupWhenEmailAlreadyInClaims(t *testing.T) {
	svc := &stubAuthSvc{
		getByExternalIDErr: apierror.NewNotFound("user not found"),
		createdUser:        &model.User{ID: "user-1"},
	}
	lookup := &stubEmailLookup{returnEmail: "should-not-be-used@example.com"}
	h := NewAuthHandler(svc, lookup)

	c, _ := newCallbackCtx("st-user-abc", "in-claims@example.com", "")
	h.Callback(c)

	if lookup.calledWith != "" {
		t.Errorf("EmailLookup should not have been called, but was called with %q", lookup.calledWith)
	}
	if svc.createdEmail != "in-claims@example.com" {
		t.Errorf("svc.Create got email %q, want %q", svc.createdEmail, "in-claims@example.com")
	}
}

// EmailLookup is optional — handler must keep working when it's not wired in
// (e.g. unit tests, embedded deployments) and gracefully when the lookup
// itself fails (transient SuperTokens core unreachable).
func TestCallback_HandlesNilOrFailingEmailLookup(t *testing.T) {
	tests := []struct {
		name   string
		lookup EmailLookup
	}{
		{name: "nil lookup", lookup: nil},
		{name: "failing lookup", lookup: &stubEmailLookup{err: errors.New("core unreachable")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubAuthSvc{
				getByExternalIDErr: apierror.NewNotFound("user not found"),
				createdUser:        &model.User{ID: "user-1"},
			}
			h := NewAuthHandler(svc, tt.lookup)

			c, rec := newCallbackCtx("st-user-abc", "", "")
			h.Callback(c)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if svc.createdEmail != "" {
				t.Errorf("svc.Create email = %q, want empty (lookup unavailable)", svc.createdEmail)
			}
		})
	}
}

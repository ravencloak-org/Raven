package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/supertokens/supertokens-golang/recipe/session"
)

// SuperTokensProvider implements AuthProvider using the SuperTokens Go SDK.
// The SDK must be initialised via InitSuperTokens before any method is called.
type SuperTokensProvider struct{}

// NewSuperTokensProvider creates a provider backed by the SuperTokens Go SDK.
// The SDK is expected to already be initialised (call InitSuperTokens once at
// startup, before creating this provider).
func NewSuperTokensProvider() *SuperTokensProvider {
	return &SuperTokensProvider{}
}

// VerifySession validates the session embedded in the request (via
// sAccessToken cookie or Authorization header) using the SuperTokens SDK.
// It returns a SessionInfo containing the SuperTokens user ID; email and name
// are populated later by the UserLookup middleware from the local database.
func (p *SuperTokensProvider) VerifySession(r *http.Request) (*SessionInfo, error) {
	// session.GetSession reads the access token from the request cookies /
	// Authorization header, verifies the JWT signature against the Core, and
	// (optionally) does a database check for revocation.
	//
	// We pass nil for res so no Set-Cookie headers are written; the SDK falls
	// back gracefully when res is nil during verification-only calls.
	dummyW := httptest.NewRecorder()
	sessionContainer, err := session.GetSession(r, dummyW, nil)
	if err != nil {
		return nil, fmt.Errorf("session verification failed: %w", err)
	}
	if sessionContainer == nil {
		return nil, fmt.Errorf("no session found")
	}

	userID := sessionContainer.GetUserID()
	email, name := extractSessionClaims(sessionContainer.GetAccessTokenPayload())

	return &SessionInfo{
		ExternalID: userID,
		Email:      email,
		Name:       name,
	}, nil
}

// RevokeSession invalidates the session identified by the request's access token.
func (p *SuperTokensProvider) RevokeSession(r *http.Request) error {
	dummyW := httptest.NewRecorder()
	sessionContainer, err := session.GetSession(r, dummyW, nil)
	if err != nil {
		return fmt.Errorf("session verification failed: %w", err)
	}
	if sessionContainer == nil {
		return fmt.Errorf("no session found")
	}
	return sessionContainer.RevokeSession()
}

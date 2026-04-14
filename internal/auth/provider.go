package auth

import "net/http"

// SessionInfo holds user identity data extracted from a verified session.
type SessionInfo struct {
	ExternalID string // Provider-specific user ID (e.g. SuperTokens user ID)
	Email      string
	Name       string
}

// Provider abstracts authentication session verification.
// Implementations may call external services (SuperTokens, etc.) to verify sessions.
type Provider interface {
	// VerifySession validates the session from the HTTP request (cookies or headers)
	// and returns the authenticated user's identity.
	// Returns an error if the session is invalid, expired, or missing.
	VerifySession(r *http.Request) (*SessionInfo, error)

	// RevokeSession invalidates the current session from the HTTP request.
	RevokeSession(r *http.Request) error
}

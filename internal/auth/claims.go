package auth

// extractSessionClaims reads email and name from the SuperTokens access token
// payload map. Returns empty strings when the keys are absent or not strings.
func extractSessionClaims(payload map[string]any) (email, name string) {
	if payload == nil {
		return "", ""
	}
	if e, ok := payload["email"].(string); ok {
		email = e
	}
	if n, ok := payload["name"].(string); ok {
		name = n
	}
	return
}

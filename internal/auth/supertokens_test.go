package auth

import (
	"testing"
)

func TestExtractSessionClaims(t *testing.T) {
	tests := []struct {
		name          string
		payload       map[string]any
		wantEmail     string
		wantName      string
	}{
		{
			name:      "both email and name present",
			payload:   map[string]any{"email": "alice@example.com", "name": "Alice"},
			wantEmail: "alice@example.com",
			wantName:  "Alice",
		},
		{
			name:      "only email present",
			payload:   map[string]any{"email": "bob@example.com"},
			wantEmail: "bob@example.com",
			wantName:  "",
		},
		{
			name:      "only name present",
			payload:   map[string]any{"name": "Charlie"},
			wantEmail: "",
			wantName:  "Charlie",
		},
		{
			name:      "nil payload",
			payload:   nil,
			wantEmail: "",
			wantName:  "",
		},
		{
			name:      "empty payload",
			payload:   map[string]any{},
			wantEmail: "",
			wantName:  "",
		},
		{
			name:      "wrong types ignored",
			payload:   map[string]any{"email": 123, "name": true},
			wantEmail: "",
			wantName:  "",
		},
		{
			name:      "extra keys ignored",
			payload:   map[string]any{"email": "d@e.com", "name": "D", "role": "admin"},
			wantEmail: "d@e.com",
			wantName:  "D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email, name := extractSessionClaims(tt.payload)
			if email != tt.wantEmail {
				t.Errorf("email = %q, want %q", email, tt.wantEmail)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

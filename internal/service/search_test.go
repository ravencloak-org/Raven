package service

import (
	"testing"
)

func TestSanitizeQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"trims whitespace", "  hello world  ", "hello world"},
		{"collapses spaces", "hello    world", "hello world"},
		{"handles tabs and newlines", "hello\t\n  world", "hello world"},
		{"empty string", "", ""},
		{"only whitespace", "   ", ""},
		{"single word", "test", "test"},
		{"preserves inner content", "multi word search query", "multi word search query"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeQuery(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"zero returns default", 0, defaultSearchLimit},
		{"negative returns default", -5, defaultSearchLimit},
		{"within range", 25, 25},
		{"at max", maxSearchLimit, maxSearchLimit},
		{"over max returns max", 200, maxSearchLimit},
		{"one is valid", 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampLimit(tt.input)
			if got != tt.want {
				t.Errorf("clampLimit(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

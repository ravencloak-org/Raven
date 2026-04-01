package service

import (
	"testing"
)

func TestStrPtr_NonEmpty(t *testing.T) {
	s := "gpt-4"
	p := strPtr(s)
	if p == nil {
		t.Fatal("expected non-nil pointer for non-empty string")
	}
	if *p != s {
		t.Errorf("expected %q, got %q", s, *p)
	}
}

func TestStrPtr_Empty(t *testing.T) {
	p := strPtr("")
	if p != nil {
		t.Errorf("expected nil pointer for empty string, got %q", *p)
	}
}

func TestSSEEventTypes(t *testing.T) {
	tests := []struct {
		event SSEEventType
		want  string
	}{
		{SSEEventToken, "token"},
		{SSEEventSources, "sources"},
		{SSEEventDone, "done"},
		{SSEEventError, "error"},
	}
	for _, tt := range tests {
		if string(tt.event) != tt.want {
			t.Errorf("SSEEventType %q != %q", tt.event, tt.want)
		}
	}
}

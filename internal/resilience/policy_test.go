package resilience

import (
	"errors"
	"testing"
	"time"
)

func TestNewPolicy_Defaults(t *testing.T) {
	p, err := NewPolicy("ai-worker")
	if err != nil {
		t.Fatalf("NewPolicy returned error: %v", err)
	}
	if p.Name != "ai-worker" {
		t.Errorf("Name = %q, want %q", p.Name, "ai-worker")
	}
	if p.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", p.Timeout)
	}
	if p.BreakerThreshold != 5 {
		t.Errorf("BreakerThreshold = %d, want 5", p.BreakerThreshold)
	}
	if p.BreakerCooldown != 30*time.Second {
		t.Errorf("BreakerCooldown = %v, want 30s", p.BreakerCooldown)
	}
	if p.BreakerHalfOpenMax != 1 {
		t.Errorf("BreakerHalfOpenMax = %d, want 1", p.BreakerHalfOpenMax)
	}
}

func TestNewPolicy_Options(t *testing.T) {
	p, err := NewPolicy("svc",
		WithTimeout(2*time.Second),
		WithBreakerThreshold(10),
		WithBreakerCooldown(15*time.Second),
		WithBreakerHalfOpenMax(3),
	)
	if err != nil {
		t.Fatalf("NewPolicy returned error: %v", err)
	}
	if p.Timeout != 2*time.Second {
		t.Errorf("Timeout = %v, want 2s", p.Timeout)
	}
	if p.BreakerThreshold != 10 {
		t.Errorf("BreakerThreshold = %d, want 10", p.BreakerThreshold)
	}
	if p.BreakerCooldown != 15*time.Second {
		t.Errorf("BreakerCooldown = %v, want 15s", p.BreakerCooldown)
	}
	if p.BreakerHalfOpenMax != 3 {
		t.Errorf("BreakerHalfOpenMax = %d, want 3", p.BreakerHalfOpenMax)
	}
}

func TestNewPolicy_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		opts []Option
	}{
		{"zero timeout", []Option{WithTimeout(0)}},
		{"negative timeout", []Option{WithTimeout(-1 * time.Second)}},
		{"zero threshold", []Option{WithBreakerThreshold(0)}},
		{"zero cooldown", []Option{WithBreakerCooldown(0)}},
		{"zero halfopen max", []Option{WithBreakerHalfOpenMax(0)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPolicy("svc", tc.opts...)
			if !errors.Is(err, ErrInvalidPolicy) {
				t.Errorf("NewPolicy err = %v, want ErrInvalidPolicy", err)
			}
		})
	}
}

func TestNewPolicy_EmptyName(t *testing.T) {
	_, err := NewPolicy("")
	if !errors.Is(err, ErrInvalidPolicy) {
		t.Errorf("NewPolicy(\"\") err = %v, want ErrInvalidPolicy", err)
	}
}

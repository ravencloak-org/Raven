package posthog_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ravencloak-org/Raven/internal/posthog"
)

func TestNewClient_NoopWhenNoAPIKey(t *testing.T) {
	c := posthog.NewClient("", "")
	if c.Enabled() {
		t.Fatal("expected client to be disabled when API key is empty")
	}

	// All methods should return nil without making HTTP calls.
	if err := c.Capture(context.Background(), "user-1", "test_event", nil); err != nil {
		t.Fatalf("Capture should be no-op: %v", err)
	}
	if err := c.Identify(context.Background(), "user-1", map[string]any{"email": "a@b.com"}); err != nil {
		t.Fatalf("Identify should be no-op: %v", err)
	}
	if err := c.Alias(context.Background(), "user-1", "anon-1"); err != nil {
		t.Fatalf("Alias should be no-op: %v", err)
	}
}

func TestCapture_SendsCorrectPayload(t *testing.T) {
	var mu sync.Mutex
	var received map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/capture" {
			t.Errorf("expected /capture, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := posthog.NewClient("phc_test123", srv.URL)
	if !c.Enabled() {
		t.Fatal("expected client to be enabled")
	}

	err := c.Capture(context.Background(), "user-42", "page_view", map[string]any{
		"url": "/home",
	})
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if received["api_key"] != "phc_test123" {
		t.Errorf("expected api_key phc_test123, got %v", received["api_key"])
	}
	if received["event"] != "page_view" {
		t.Errorf("expected event page_view, got %v", received["event"])
	}
	if received["distinct_id"] != "user-42" {
		t.Errorf("expected distinct_id user-42, got %v", received["distinct_id"])
	}
	props, ok := received["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties should be a map")
	}
	if props["url"] != "/home" {
		t.Errorf("expected url /home, got %v", props["url"])
	}
}

func TestIdentify_SendsIdentifyEvent(t *testing.T) {
	var mu sync.Mutex
	var received map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := posthog.NewClient("phc_test456", srv.URL)
	err := c.Identify(context.Background(), "user-99", map[string]any{
		"email": "user@example.com",
		"name":  "Test User",
	})
	if err != nil {
		t.Fatalf("Identify failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if received["event"] != "$identify" {
		t.Errorf("expected event $identify, got %v", received["event"])
	}
	if received["distinct_id"] != "user-99" {
		t.Errorf("expected distinct_id user-99, got %v", received["distinct_id"])
	}
}

func TestAlias_SendsCreateAliasEvent(t *testing.T) {
	var mu sync.Mutex
	var received map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		_ = json.Unmarshal(body, &received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := posthog.NewClient("phc_test789", srv.URL)
	err := c.Alias(context.Background(), "user-10", "anon-session-abc")
	if err != nil {
		t.Fatalf("Alias failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if received["event"] != "$create_alias" {
		t.Errorf("expected event $create_alias, got %v", received["event"])
	}
	if received["distinct_id"] != "user-10" {
		t.Errorf("expected distinct_id user-10, got %v", received["distinct_id"])
	}
	props, ok := received["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties should be a map")
	}
	if props["alias"] != "anon-session-abc" {
		t.Errorf("expected alias anon-session-abc, got %v", props["alias"])
	}
}

func TestCapture_ReturnsErrorOnServerFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := posthog.NewClient("phc_test", srv.URL)
	err := c.Capture(context.Background(), "user-1", "test", nil)
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestCapture_ReturnsErrorOnCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := posthog.NewClient("phc_test", srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := c.Capture(ctx, "user-1", "test", nil)
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

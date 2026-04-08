package meta_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ravencloak-org/Raven/pkg/meta"
)

func TestSendSDPAnswer_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/phone-123/calls/call-abc" {
			t.Errorf("path = %s, want /phone-123/calls/call-abc", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("auth header = %q, want 'Bearer test-token'", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content-type = %q, want 'application/json'", r.Header.Get("Content-Type"))
		}

		var body meta.SendSDPAnswerRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.SDPAnswer != "v=0\r\nanswer" {
			t.Errorf("sdp = %q, want 'v=0\\r\\nanswer'", body.SDPAnswer)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meta.SendSDPAnswerResponse{Success: true}) //nolint:errcheck
	}))
	defer ts.Close()

	c := meta.NewClient(meta.WithBaseURL(ts.URL))
	resp, err := c.SendSDPAnswer(context.Background(), "test-token", "phone-123", "call-abc", "v=0\r\nanswer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestSendSDPAnswer_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"error": map[string]any{
				"message":    "Invalid SDP",
				"type":       "OAuthException",
				"code":       100,
				"fbtrace_id": "trace-123",
			},
		})
	}))
	defer ts.Close()

	c := meta.NewClient(meta.WithBaseURL(ts.URL))
	_, err := c.SendSDPAnswer(context.Background(), "token", "phone", "call", "bad")
	if err == nil {
		t.Fatal("expected error for API error response")
	}
}

func TestGetCallStatus_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/phone-123/calls/call-xyz" {
			t.Errorf("path = %s, want /phone-123/calls/call-xyz", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meta.CallStatusResponse{ID: "call-xyz", Status: "connected"}) //nolint:errcheck
	}))
	defer ts.Close()

	c := meta.NewClient(meta.WithBaseURL(ts.URL))
	resp, err := c.GetCallStatus(context.Background(), "token", "phone-123", "call-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "connected" {
		t.Errorf("status = %q, want 'connected'", resp.Status)
	}
}

func TestGetCallStatus_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error")) //nolint:errcheck
	}))
	defer ts.Close()

	c := meta.NewClient(meta.WithBaseURL(ts.URL))
	_, err := c.GetCallStatus(context.Background(), "token", "phone", "call")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

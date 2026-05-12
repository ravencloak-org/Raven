package mail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResendSender_Send_PostsToAPI(t *testing.T) {
	var got struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer test-key"; got != want {
			t.Errorf("auth header: got %q want %q", got, want)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg-123"}`))
	}))
	defer srv.Close()

	s := &ResendSender{
		APIKey: "test-key",
		From:   "noreply@example.org",
		client: srv.Client(),
		url:    srv.URL,
	}

	err := s.Send(context.Background(), Message{
		To:      "user@example.com",
		Subject: "Hi",
		Text:    "Hello",
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if got.From != "noreply@example.org" {
		t.Errorf("From: got %q", got.From)
	}
	if !strings.Contains(strings.Join(got.To, ","), "user@example.com") {
		t.Errorf("To: got %v", got.To)
	}
	if got.Subject != "Hi" {
		t.Errorf("Subject: got %q", got.Subject)
	}
}

func TestResendSender_Send_PropagatesNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	s := &ResendSender{
		APIKey: "bad",
		From:   "noreply@example.org",
		client: srv.Client(),
		url:    srv.URL,
	}
	err := s.Send(context.Background(), Message{To: "u@e.com", Subject: "s", Text: "t"})
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected error to mention status code: %v", err)
	}
}

func TestResendSender_Send_RequiresAPIKey(t *testing.T) {
	s := &ResendSender{APIKey: "", From: "noreply@example.org"}
	err := s.Send(context.Background(), Message{To: "u@e.com", Subject: "s", Text: "t"})
	if err == nil {
		t.Fatal("expected error when API key is empty")
	}
}

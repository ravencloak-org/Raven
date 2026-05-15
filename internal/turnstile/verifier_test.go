package turnstile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifier_Verify_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if got, want := r.Form.Get("secret"), "secret-key"; got != want {
			t.Errorf("secret: got %q want %q", got, want)
		}
		if got, want := r.Form.Get("response"), "good-token"; got != want {
			t.Errorf("response: got %q want %q", got, want)
		}
		if got, want := r.Form.Get("remoteip"), "1.2.3.4"; got != want {
			t.Errorf("remoteip: got %q want %q", got, want)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer srv.Close()

	v := &Verifier{SecretKey: "secret-key", url: srv.URL, client: srv.Client()}
	ok, err := v.Verify(context.Background(), "good-token", "1.2.3.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected success")
	}
}

func TestVerifier_Verify_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":     false,
			"error-codes": []string{"invalid-input-response"},
		})
	}))
	defer srv.Close()

	v := &Verifier{SecretKey: "k", url: srv.URL, client: srv.Client()}
	ok, err := v.Verify(context.Background(), "bad-token", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected failure")
	}
}

func TestVerifier_Verify_OmitsRemoteIPWhenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if _, set := r.Form["remoteip"]; set {
			t.Errorf("remoteip should be absent when caller passes empty string")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer srv.Close()

	v := &Verifier{SecretKey: "k", url: srv.URL, client: srv.Client()}
	_, _ = v.Verify(context.Background(), "tok", "")
}

func TestVerifier_Verify_RequiresSecret(t *testing.T) {
	v := &Verifier{SecretKey: ""}
	_, err := v.Verify(context.Background(), "tok", "")
	if err == nil {
		t.Fatal("expected error when secret is empty")
	}
}

package storage_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ravencloak-org/Raven/internal/storage"
)

func TestSeaweedFSClient_Upload_Success(t *testing.T) {
	// Mock the SeaweedFS master and volume server.
	volumeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		// Verify it's a multipart upload.
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("expected multipart/form-data, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"name":"test.pdf","size":100}`))
	}))
	defer volumeServer.Close()

	// Extract host:port from volume server URL (strip http://).
	volumeAddr := strings.TrimPrefix(volumeServer.URL, "http://")

	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dir/assign" && r.Method == http.MethodPost {
			resp := map[string]any{
				"fid":       "3,01637037d6",
				"url":       volumeAddr,
				"publicUrl": volumeAddr,
				"count":     1,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer masterServer.Close()

	client := storage.NewSeaweedFSClient(masterServer.URL, masterServer.Client())
	fid, err := client.Upload(context.Background(), "test.pdf", strings.NewReader("file content"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	if fid != "3,01637037d6" {
		t.Errorf("expected fid '3,01637037d6', got '%s'", fid)
	}
}

func TestSeaweedFSClient_Upload_AssignError(t *testing.T) {
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	}))
	defer masterServer.Close()

	client := storage.NewSeaweedFSClient(masterServer.URL, masterServer.Client())
	_, err := client.Upload(context.Background(), "test.pdf", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unexpected status 500") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSeaweedFSClient_Upload_AssignErrorField(t *testing.T) {
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "no free volumes",
		})
	}))
	defer masterServer.Close()

	client := storage.NewSeaweedFSClient(masterServer.URL, masterServer.Client())
	_, err := client.Upload(context.Background(), "test.pdf", strings.NewReader("data"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no free volumes") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSeaweedFSClient_Download_Success(t *testing.T) {
	volumeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		_, _ = w.Write([]byte("file content"))
	}))
	defer volumeServer.Close()

	volumeAddr := strings.TrimPrefix(volumeServer.URL, "http://")

	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dir/lookup" {
			resp := map[string]any{
				"locations": []map[string]string{
					{"url": volumeAddr, "publicUrl": volumeAddr},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer masterServer.Close()

	client := storage.NewSeaweedFSClient(masterServer.URL, masterServer.Client())
	rc, err := client.Download(context.Background(), "3,01637037d6")
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, _ := io.ReadAll(rc)
	if string(data) != "file content" {
		t.Errorf("expected 'file content', got '%s'", string(data))
	}
}

func TestSeaweedFSClient_Download_LookupError(t *testing.T) {
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": "volume not found",
		})
	}))
	defer masterServer.Close()

	client := storage.NewSeaweedFSClient(masterServer.URL, masterServer.Client())
	_, err := client.Download(context.Background(), "999,abcdef")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "volume not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSeaweedFSClient_Delete_Success(t *testing.T) {
	volumeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer volumeServer.Close()

	volumeAddr := strings.TrimPrefix(volumeServer.URL, "http://")

	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/dir/lookup" {
			resp := map[string]any{
				"locations": []map[string]string{
					{"url": volumeAddr, "publicUrl": volumeAddr},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.NotFound(w, r)
	}))
	defer masterServer.Close()

	client := storage.NewSeaweedFSClient(masterServer.URL, masterServer.Client())
	err := client.Delete(context.Background(), "3,01637037d6")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
}

func TestSeaweedFSClient_Delete_LookupNoLocations(t *testing.T) {
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"locations": []map[string]string{},
		})
	}))
	defer masterServer.Close()

	client := storage.NewSeaweedFSClient(masterServer.URL, masterServer.Client())
	err := client.Delete(context.Background(), "3,01637037d6")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no locations") {
		t.Errorf("unexpected error message: %v", err)
	}
}

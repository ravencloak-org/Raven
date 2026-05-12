package account

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeExportRepo struct{}

func (fakeExportRepo) ExportUser(_ context.Context, userID string) (UserExport, error) {
	return UserExport{
		UserID: userID,
		Email:  "u@example.com",
		Rows: map[string][]map[string]any{
			"messages": {{"id": float64(1), "text": "hi"}},
		},
	}, nil
}

func (fakeExportRepo) ScheduleDelete(_ context.Context, _ string) error {
	return nil
}

func TestExportHandler_WritesJSON(t *testing.T) {
	h := NewDSARHandler(fakeExportRepo{}, nil)
	req := httptest.NewRequestWithContext(
		WithUserID(context.Background(), "user-1"),
		http.MethodGet, "/account/export", nil,
	)
	w := httptest.NewRecorder()

	h.Export(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("unexpected content-type: %s", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "raven-export.json") {
		t.Fatalf("missing or wrong Content-Disposition: %s", cd)
	}

	var out UserExport
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.UserID != "user-1" {
		t.Fatalf("UserID: got %s", out.UserID)
	}
	if len(out.Rows["messages"]) != 1 {
		t.Fatalf("expected 1 message row")
	}
}

func TestExportHandler_UnauthenticatedReturns401(t *testing.T) {
	h := NewDSARHandler(fakeExportRepo{}, nil)
	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodGet, "/account/export", nil) // no user_id context
	w := httptest.NewRecorder()

	h.Export(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestUserIDFrom_ReturnsFalseWhenMissing(t *testing.T) {
	_, ok := UserIDFrom(context.Background())
	if ok {
		t.Fatal("expected ok=false for empty context")
	}
}

func TestUserIDFrom_ReturnsTrueWhenSet(t *testing.T) {
	ctx := WithUserID(context.Background(), "u-1")
	id, ok := UserIDFrom(ctx)
	if !ok || id != "u-1" {
		t.Fatalf("got %q ok=%v, want u-1 true", id, ok)
	}
}

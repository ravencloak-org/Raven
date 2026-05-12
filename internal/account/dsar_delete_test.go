package account

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ravencloak-org/Raven/internal/mail"
)

type fakeDeleteRepo struct {
	scheduled string
}

func (f *fakeDeleteRepo) ExportUser(_ context.Context, id string) (UserExport, error) {
	return UserExport{UserID: id}, nil
}

func (f *fakeDeleteRepo) ScheduleDelete(_ context.Context, id string) error {
	f.scheduled = id
	return nil
}

type recordingMailer struct {
	sent []mail.Message
}

func (r *recordingMailer) Send(_ context.Context, m mail.Message) error {
	r.sent = append(r.sent, m)
	return nil
}

func TestDeleteHandler_SchedulesAndEmails(t *testing.T) {
	repo := &fakeDeleteRepo{}
	m := &recordingMailer{}
	h := NewDSARHandler(repo, m)

	ctx := WithUserEmail(WithUserID(context.Background(), "user-2"), "u@e.com")
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/account/delete", nil)
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if repo.scheduled != "user-2" {
		t.Fatalf("expected schedule for user-2, got %q", repo.scheduled)
	}
	if len(m.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(m.sent))
	}
	if m.sent[0].To != "u@e.com" {
		t.Fatalf("expected To u@e.com, got %s", m.sent[0].To)
	}
	if !strings.Contains(strings.ToLower(m.sent[0].Subject), "delete") {
		t.Fatalf("subject should mention delete: %s", m.sent[0].Subject)
	}
}

func TestDeleteHandler_NoMailerStillSchedules(t *testing.T) {
	repo := &fakeDeleteRepo{}
	h := NewDSARHandler(repo, nil)

	ctx := WithUserID(context.Background(), "user-3")
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/account/delete", nil)
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if repo.scheduled != "user-3" {
		t.Fatalf("expected schedule for user-3, got %q", repo.scheduled)
	}
}

func TestDeleteHandler_UnauthenticatedReturns401(t *testing.T) {
	h := NewDSARHandler(&fakeDeleteRepo{}, nil)
	req := httptest.NewRequestWithContext(context.Background(),
		http.MethodPost, "/account/delete", nil)
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

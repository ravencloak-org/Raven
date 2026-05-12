package account

import (
	"context"
	"testing"
	"time"
)

type stubRetentionRepo struct {
	users   []InactiveUser
	warned  []string
	deleted []string
}

func (s *stubRetentionRepo) InactiveUsers(_ context.Context, _ time.Time) ([]InactiveUser, error) {
	return s.users, nil
}

func (s *stubRetentionRepo) MarkWarned(_ context.Context, id string) error {
	s.warned = append(s.warned, id)
	return nil
}

func (s *stubRetentionRepo) HardDelete(_ context.Context, id string) error {
	s.deleted = append(s.deleted, id)
	return nil
}

func TestRunOnce_WarnsAtDay23DeletesAt30(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRetentionRepo{
		users: []InactiveUser{
			{ID: "warn-me", Email: "w@e.com", LastActive: now.Add(-25 * 24 * time.Hour)},
			{ID: "purge-me", Email: "p@e.com", LastActive: now.Add(-31 * 24 * time.Hour)},
			{ID: "edge-23", Email: "edge23@e.com", LastActive: now.Add(-23*24*time.Hour - time.Hour)},
		},
	}
	mailer := &recordingMailer{}
	p := NewRetentionPurger(repo, mailer)

	if err := p.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := len(repo.deleted), 1; got != want {
		t.Fatalf("deleted: got %d want %d (%v)", got, want, repo.deleted)
	}
	if repo.deleted[0] != "purge-me" {
		t.Fatalf("deleted wrong user: %s", repo.deleted[0])
	}

	if got, want := len(repo.warned), 2; got != want {
		t.Fatalf("warned: got %d want %d (%v)", got, want, repo.warned)
	}

	// One email per warned user; none for deleted ones.
	if got, want := len(mailer.sent), 2; got != want {
		t.Fatalf("sent: got %d want %d", got, want)
	}
	recipients := map[string]bool{}
	for _, m := range mailer.sent {
		recipients[m.To] = true
	}
	if !recipients["w@e.com"] || !recipients["edge23@e.com"] {
		t.Fatalf("missing warning recipients: %v", recipients)
	}
}

func TestRunOnce_EmptyInactiveListIsNoop(t *testing.T) {
	repo := &stubRetentionRepo{}
	mailer := &recordingMailer{}
	p := NewRetentionPurger(repo, mailer)

	if err := p.RunOnce(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.warned) != 0 || len(repo.deleted) != 0 || len(mailer.sent) != 0 {
		t.Fatalf("expected no side effects")
	}
}

func TestRunOnce_NoMailerStillWarnsAndDeletes(t *testing.T) {
	now := time.Now().UTC()
	repo := &stubRetentionRepo{
		users: []InactiveUser{
			{ID: "warn-me", Email: "w@e.com", LastActive: now.Add(-25 * 24 * time.Hour)},
			{ID: "purge-me", Email: "p@e.com", LastActive: now.Add(-31 * 24 * time.Hour)},
		},
	}
	p := NewRetentionPurger(repo, nil)

	if err := p.RunOnce(context.Background(), now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.warned) != 1 || repo.warned[0] != "warn-me" {
		t.Fatalf("warned: %v", repo.warned)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != "purge-me" {
		t.Fatalf("deleted: %v", repo.deleted)
	}
}

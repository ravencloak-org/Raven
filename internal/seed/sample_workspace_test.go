package seed

import (
	"context"
	"errors"
	"testing"
)

type fakeWS struct {
	created  string
	messages []string
}

func (f *fakeWS) CreateWorkspace(_ context.Context, ownerID, name string) (string, error) {
	f.created = ownerID + ":" + name
	return "ws-1", nil
}

func (f *fakeWS) InsertMessage(_ context.Context, ws, role, text string) error {
	f.messages = append(f.messages, ws+"|"+role+"|"+text)
	return nil
}

func TestSampleWorkspace_CreatesWorkspaceAndMessages(t *testing.T) {
	repo := &fakeWS{}
	if err := SampleWorkspace(context.Background(), repo, "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.created != "user-1:Sample workspace" {
		t.Fatalf("workspace creation: got %q", repo.created)
	}
	if len(repo.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(repo.messages))
	}
	// Both messages should be tagged with the workspace id returned by
	// CreateWorkspace (ws-1) and use the role from the fixture.
	for i, m := range repo.messages {
		if !startsWith(m, "ws-1|") {
			t.Fatalf("message %d not tagged with workspace id: %s", i, m)
		}
	}
}

type errWS struct{}

func (errWS) CreateWorkspace(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("db down")
}

func (errWS) InsertMessage(_ context.Context, _, _, _ string) error {
	return nil
}

func TestSampleWorkspace_PropagatesCreateError(t *testing.T) {
	if err := SampleWorkspace(context.Background(), errWS{}, "user-1"); err == nil {
		t.Fatal("expected error from CreateWorkspace failure")
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

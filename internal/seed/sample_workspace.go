// Package seed installs starter content for new users so the demo
// looks alive immediately after signup. Driven by an embedded JSON
// fixture; the SuperTokens user-create hook calls SampleWorkspace
// once per new account.
package seed

import (
	"context"
	_ "embed"
	"encoding/json"
)

//go:embed fixtures/sample_workspace.json
var sampleWorkspaceJSON []byte

type fixture struct {
	WorkspaceName string `json:"workspace_name"`
	Messages      []struct {
		Role string `json:"role"`
		Text string `json:"text"`
	} `json:"messages"`
}

// WorkspaceSeeder is the persistence surface required by
// SampleWorkspace. A real implementation will wrap the workspace +
// chat tables; tests can pass a fake.
type WorkspaceSeeder interface {
	CreateWorkspace(ctx context.Context, ownerID, name string) (string, error)
	InsertMessage(ctx context.Context, workspaceID, role, text string) error
}

// SampleWorkspace creates a starter workspace for ownerID and
// populates it with the messages defined in
// internal/seed/fixtures/sample_workspace.json.
//
// Errors from CreateWorkspace propagate. Errors from individual
// InsertMessage calls also propagate (so the caller can decide whether
// to retry or treat the seed as best-effort).
func SampleWorkspace(ctx context.Context, s WorkspaceSeeder, ownerID string) error {
	var f fixture
	if err := json.Unmarshal(sampleWorkspaceJSON, &f); err != nil {
		return err
	}
	ws, err := s.CreateWorkspace(ctx, ownerID, f.WorkspaceName)
	if err != nil {
		return err
	}
	for _, m := range f.Messages {
		if err := s.InsertMessage(ctx, ws, m.Role, m.Text); err != nil {
			return err
		}
	}
	return nil
}

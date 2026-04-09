package testutil

import (
	"github.com/google/uuid"
	"github.com/ravencloak-org/Raven/internal/model"
)

// NewOrg creates a test Organization with sensible defaults.
func NewOrg(overrides ...func(*model.Organization)) *model.Organization {
	org := &model.Organization{
		ID:     uuid.NewString(),
		Name:   "Test Org",
		Slug:   "test-org",
		Status: model.OrgStatusActive,
	}
	for _, o := range overrides {
		o(org)
	}
	return org
}

// NewWorkspace creates a test Workspace belonging to orgID.
func NewWorkspace(orgID string, overrides ...func(*model.Workspace)) *model.Workspace {
	ws := &model.Workspace{
		ID:    uuid.NewString(),
		OrgID: orgID,
		Name:  "Test Workspace",
		Slug:  "test-workspace",
	}
	for _, o := range overrides {
		o(ws)
	}
	return ws
}

// NewKnowledgeBase creates a test KnowledgeBase belonging to workspaceID.
func NewKnowledgeBase(workspaceID string, overrides ...func(*model.KnowledgeBase)) *model.KnowledgeBase {
	kb := &model.KnowledgeBase{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        "Test KB",
		Slug:        "test-kb",
		Status:      model.KBStatusActive,
	}
	for _, o := range overrides {
		o(kb)
	}
	return kb
}

// NewAPIKey creates a test APIKey scoped to a workspace and knowledge base.
func NewAPIKey(workspaceID, kbID string, overrides ...func(*model.APIKey)) *model.APIKey {
	key := &model.APIKey{
		ID:              uuid.NewString(),
		WorkspaceID:     workspaceID,
		KnowledgeBaseID: kbID,
		Name:            "Test Key",
		KeyHash:         "testhash",
		KeyPrefix:       "rv_test",
		Status:          model.APIKeyStatusActive,
	}
	for _, o := range overrides {
		o(key)
	}
	return key
}

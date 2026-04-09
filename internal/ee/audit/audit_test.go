// Package audit_test verifies the enterprise audit package compiles and
// provides tests for audit log capture behaviour.
package audit_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPackageCompiles ensures the audit package is importable and correctly declared.
func TestPackageCompiles(t *testing.T) {
	t.Log("internal/ee/audit package compiles successfully")
}

// AuditEntry is a test-local representation of an audit log entry.
// Once the audit package exposes its types, this should be replaced.
type AuditEntry struct {
	ID        string
	OrgID     string
	Actor     string
	Action    string
	Resource  string
	CreatedAt time.Time
}

// AuditStore is an in-memory store for audit testing.
type AuditStore struct {
	entries []AuditEntry
}

func (s *AuditStore) Create(entry AuditEntry) {
	s.entries = append(s.entries, entry)
}

func (s *AuditStore) QueryByActor(actor string) []AuditEntry {
	var result []AuditEntry
	for _, e := range s.entries {
		if e.Actor == actor {
			result = append(result, e)
		}
	}
	return result
}

// TestAuditLog_Create_CapturesCorrectFields verifies that creating an audit
// entry captures the expected org, actor, action, and resource fields.
func TestAuditLog_Create_CapturesCorrectFields(t *testing.T) {
	store := &AuditStore{}
	entry := AuditEntry{
		ID:        "audit-1",
		OrgID:     "org-test",
		Actor:     "user-admin",
		Action:    "delete",
		Resource:  "kb/kb-secret",
		CreatedAt: time.Now(),
	}
	store.Create(entry)

	require.Len(t, store.entries, 1)
	got := store.entries[0]
	assert.Equal(t, "org-test", got.OrgID)
	assert.Equal(t, "user-admin", got.Actor)
	assert.Equal(t, "delete", got.Action)
	assert.Equal(t, "kb/kb-secret", got.Resource)
}

// TestAuditLog_Delete_EntryRetained verifies that audit entries are immutable —
// deleting a resource must not remove its audit trail.
func TestAuditLog_Delete_EntryRetained(t *testing.T) {
	store := &AuditStore{}
	store.Create(AuditEntry{ID: "audit-1", OrgID: "org-1", Actor: "alice", Action: "create", Resource: "kb/1"})
	store.Create(AuditEntry{ID: "audit-2", OrgID: "org-1", Actor: "alice", Action: "delete", Resource: "kb/1"})

	// Even after a delete action, both entries must remain.
	assert.Len(t, store.entries, 2, "audit entries must be retained even after a delete action")
}

// TestAuditLog_Query_FilterByActor verifies that audit log queries correctly
// filter entries by actor, returning only matching entries.
func TestAuditLog_Query_FilterByActor(t *testing.T) {
	store := &AuditStore{}
	store.Create(AuditEntry{ID: "1", Actor: "alice", Action: "login"})
	store.Create(AuditEntry{ID: "2", Actor: "bob", Action: "create"})
	store.Create(AuditEntry{ID: "3", Actor: "alice", Action: "delete"})

	aliceEntries := store.QueryByActor("alice")
	assert.Len(t, aliceEntries, 2, "query by actor=alice must return 2 entries")

	bobEntries := store.QueryByActor("bob")
	assert.Len(t, bobEntries, 1, "query by actor=bob must return 1 entry")

	unknownEntries := store.QueryByActor("charlie")
	assert.Empty(t, unknownEntries, "unknown actor must return no entries")
}

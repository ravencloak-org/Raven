package model

// kbStatusCapabilities is the single source of truth for "what can a KB in
// this status accept?". A new KBStatus value MUST get a row here, or the
// gate's accessors will silently report "no" — fine as a safe default, but
// the test suite (TestKBStatusGate_TableCoversAllStatuses) compiles only
// when every declared KBStatus has an entry.
//
// Keeping the table package-private forces every consumer through the
// KBStatusGate methods so the policy stays in one file. If you find
// yourself wanting to read this map outside the package, add a method
// instead — that's the abstraction earning its keep.
var kbStatusCapabilities = map[KBStatus]struct {
	read    bool
	ingest  bool
	chat    bool
	publish bool
	// reason is the human-facing explanation shown to chat / widget users
	// when a frozen state blocks an interaction. Empty for states that
	// either allow everything (active) or are invisible to end users
	// (archived — they cannot reach the surface at all).
	reason string
}{
	KBStatusActive: {
		read:    true,
		ingest:  true,
		chat:    true,
		publish: true,
		reason:  "",
	},
	KBStatusReadOnlyPrivate: {
		// ADR-0004: downgraded private KBs stay reachable so users can
		// query the existing corpus, but every write path returns 409
		// with a machine-readable kb_frozen code until the user upgrades
		// back or publishes / deletes the KB.
		read:    true,
		ingest:  false,
		chat:    true,
		publish: false,
		reason:  "This knowledge base is frozen because the workspace is on the Free Plan. Upgrade, publish, or delete it to resume ingestion.",
	},
	KBStatusDMCAPending: {
		// ADR-0006 + ADR-0008: legal hold during the 14-day counter-
		// notice window. The publisher still owns the row, so the
		// metadata stays visible, but every user-facing surface (chat,
		// widget, ingestion, publish) is locked.
		read:    true,
		ingest:  false,
		chat:    false,
		publish: false,
		reason:  "This knowledge base is temporarily unavailable while a copyright notice is reviewed.",
	},
	KBStatusArchived: {
		// Soft-deleted. Everything is closed; the row should never
		// reach a user-facing call site because the read-path filters
		// already drop it. The gate stays consistent regardless so a
		// stale handle returns a clean refusal instead of a panic.
		read:    false,
		ingest:  false,
		chat:    false,
		publish: false,
		reason:  "This knowledge base has been archived.",
	},
}

// KBStatusGate is the canonical helper for "can this KB accept X right now?".
// All status semantics live here so feature code can ask `gate.CanIngest()`
// instead of sprinkling `if kb.Status == "active"` checks across the
// codebase. Add a new state by extending kbStatusCapabilities, not by
// branching on the enum literal at the call site.
type KBStatusGate struct {
	Status KBStatus
}

// NewKBStatusGate builds a gate for the given status. Unknown statuses fall
// through to a closed-by-default capability set — same as Archived — so
// future enum additions cannot silently widen access before the table is
// updated.
func NewKBStatusGate(s KBStatus) KBStatusGate {
	return KBStatusGate{Status: s}
}

// CanRead reports whether the KB metadata + existing content is visible.
func (g KBStatusGate) CanRead() bool {
	return kbStatusCapabilities[g.Status].read
}

// CanIngest reports whether new sources, documents, or chunks may be
// added. Settings updates that affect what the KB serves also gate on
// this, since "frozen" includes "you cannot change what it answers with".
func (g KBStatusGate) CanIngest() bool {
	return kbStatusCapabilities[g.Status].ingest
}

// CanChat reports whether the chat / widget completion paths may run
// against this KB. Read-only-private allows chat (the corpus is
// queryable); DMCA-pending does not (legal hold).
func (g KBStatusGate) CanChat() bool {
	return kbStatusCapabilities[g.Status].chat
}

// CanPublish reports whether the publish-to-Marketplace action is allowed.
// Only active KBs may publish — every other state is some flavour of
// "frozen" and publishing during freeze would be a footgun.
func (g KBStatusGate) CanPublish() bool {
	return kbStatusCapabilities[g.Status].publish
}

// PublicReason returns a human-readable explanation suitable for surfacing
// in chat / widget error payloads. Empty string when the KB is fully
// available (no message to show).
func (g KBStatusGate) PublicReason() string {
	return kbStatusCapabilities[g.Status].reason
}

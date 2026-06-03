package model

import "testing"

// TestKBStatusConstants pins the string values to the kb_status Postgres
// enum labels added in migrations 00001 and 00049 (issue #725). A typo
// here would surface as zero-row writes at runtime; we catch it at test
// time instead.
func TestKBStatusConstants(t *testing.T) {
	cases := map[KBStatus]string{
		KBStatusActive:          "active",
		KBStatusArchived:        "archived",
		KBStatusReadOnlyPrivate: "read_only_private",
		KBStatusDMCAPending:     "dmca_pending",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("KBStatus constant: want %q, got %q", want, got)
		}
	}
}

// TestKBStatusGate_CapabilityMatrix exhaustively covers every status × every
// gate method against the policy table. If a future status row gets added
// without an entry in the matrix here the build still passes — but the
// per-status sub-tests below pin the contract so behaviour stays
// observable.
func TestKBStatusGate_CapabilityMatrix(t *testing.T) {
	type want struct {
		read, ingest, chat, publish bool
		reasonNonEmpty              bool
	}
	cases := map[KBStatus]want{
		KBStatusActive: {
			read: true, ingest: true, chat: true, publish: true,
			reasonNonEmpty: false,
		},
		KBStatusReadOnlyPrivate: {
			// Free Plan freeze: reads + chat stay open, ingestion blocked.
			read: true, ingest: false, chat: true, publish: false,
			reasonNonEmpty: true,
		},
		KBStatusDMCAPending: {
			// Legal hold: reads stay open (publisher owns the row) but
			// every interactive surface is locked.
			read: true, ingest: false, chat: false, publish: false,
			reasonNonEmpty: true,
		},
		KBStatusArchived: {
			// Soft-deleted: everything closed.
			read: false, ingest: false, chat: false, publish: false,
			reasonNonEmpty: true,
		},
	}

	for status, w := range cases {
		status, w := status, w
		t.Run(string(status), func(t *testing.T) {
			gate := NewKBStatusGate(status)
			if got := gate.CanRead(); got != w.read {
				t.Errorf("CanRead(%s): want %v, got %v", status, w.read, got)
			}
			if got := gate.CanIngest(); got != w.ingest {
				t.Errorf("CanIngest(%s): want %v, got %v", status, w.ingest, got)
			}
			if got := gate.CanChat(); got != w.chat {
				t.Errorf("CanChat(%s): want %v, got %v", status, w.chat, got)
			}
			if got := gate.CanPublish(); got != w.publish {
				t.Errorf("CanPublish(%s): want %v, got %v", status, w.publish, got)
			}
			got := gate.PublicReason()
			if w.reasonNonEmpty && got == "" {
				t.Errorf("PublicReason(%s): expected non-empty, got empty", status)
			}
			if !w.reasonNonEmpty && got != "" {
				t.Errorf("PublicReason(%s): expected empty, got %q", status, got)
			}
		})
	}
}

// TestKBStatusGate_UnknownStatus closes a footgun: an enum value the Go
// code hasn't been taught about must NOT silently grant capabilities.
// Defaulting to "deny" means an unknown status reaches the call site as a
// 409 / 423, not a permissive 200.
func TestKBStatusGate_UnknownStatus(t *testing.T) {
	gate := NewKBStatusGate(KBStatus("totally_made_up"))
	if gate.CanRead() {
		t.Error("unknown status: CanRead must default to false")
	}
	if gate.CanIngest() {
		t.Error("unknown status: CanIngest must default to false")
	}
	if gate.CanChat() {
		t.Error("unknown status: CanChat must default to false")
	}
	if gate.CanPublish() {
		t.Error("unknown status: CanPublish must default to false")
	}
}

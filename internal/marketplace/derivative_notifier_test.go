package marketplace_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ravencloak-org/Raven/internal/mail"
	"github.com/ravencloak-org/Raven/internal/marketplace"
)

// TestMailNotifier_FormatsBody is a small smoke test on the default
// production notifier wiring — the wording isn't asserted character-by-
// character because that would make every copy edit a code change, but
// the four substitution points (recipient, source name, source org,
// derivative name) are checked so a future refactor that drops one
// from the template fails loudly.
func TestMailNotifier_FormatsBody(t *testing.T) {
	sender := &mail.NoopSender{}
	n := marketplace.NewMailNotifier(sender)
	ctx := context.Background()
	if err := n.NotifyDerivativeTakedown(ctx, marketplace.DerivativeNotice{
		RecipientEmail:       "owner@example.com",
		DerivativeKBID:       uuid.New(),
		DerivativeKBName:     "My Recipe Fork",
		SourceKBID:           uuid.New(),
		SourceKBName:         "Original Recipes",
		SourceOrgDisplayName: "Upstream Co",
		Reason:               "trademark dispute",
	}); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sender.Sent) != 1 {
		t.Fatalf("Sent: want 1, got %d", len(sender.Sent))
	}
	m := sender.Sent[0]
	if m.To != "owner@example.com" {
		t.Errorf("To: %q", m.To)
	}
	for _, want := range []string{
		"Original Recipes",
		"Upstream Co",
		"My Recipe Fork",
		"trademark dispute",
	} {
		if !contains(m.Subject+m.Text, want) {
			t.Errorf("body missing %q in subject+text", want)
		}
	}
}

func TestMailNotifier_NilSenderFallsBackToNoop(t *testing.T) {
	n := marketplace.NewMailNotifier(nil)
	if err := n.NotifyDerivativeTakedown(context.Background(), marketplace.DerivativeNotice{
		RecipientEmail: "owner@example.com",
		SourceKBName:   "X",
	}); err != nil {
		t.Fatalf("nil-sender path should not error, got: %v", err)
	}
}

func TestMailNotifier_EmptyReasonFallback(t *testing.T) {
	sender := &mail.NoopSender{}
	n := marketplace.NewMailNotifier(sender)
	if err := n.NotifyDerivativeTakedown(context.Background(), marketplace.DerivativeNotice{
		RecipientEmail: "owner@example.com",
		SourceKBName:   "X",
	}); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if len(sender.Sent) != 1 {
		t.Fatalf("Sent: want 1, got %d", len(sender.Sent))
	}
	if !contains(sender.Sent[0].Text, "(no reason recorded)") {
		t.Errorf("expected empty-reason fallback marker in body, got: %s", sender.Sent[0].Text)
	}
}

// recordingNotifier captures every call so registry tests can assert
// ordering and arg propagation.
type recordingNotifier struct {
	calls []marketplace.DerivativeNotice
	err   error
}

func (r *recordingNotifier) NotifyDerivativeTakedown(_ context.Context, n marketplace.DerivativeNotice) error {
	r.calls = append(r.calls, n)
	return r.err
}

func TestRegisterOnTakedownCreated_FanOut(t *testing.T) {
	marketplace.ResetOnTakedownCreatedForTest()
	var got1, got2 marketplace.Takedown
	marketplace.RegisterOnTakedownCreated(func(_ context.Context, td marketplace.Takedown, _ string) error {
		got1 = td
		return nil
	})
	marketplace.RegisterOnTakedownCreated(func(_ context.Context, td marketplace.Takedown, _ string) error {
		got2 = td
		return nil
	})
	td := marketplace.Takedown{
		ID:         uuid.New(),
		TargetKBID: uuid.New(),
		Source:     marketplace.TakedownSourceAdmin,
		CreatedAt:  time.Now().UTC(),
	}
	if err := marketplace.DispatchOnTakedownCreated(context.Background(), td, "policy violation"); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got1.ID != td.ID || got2.ID != td.ID {
		t.Errorf("hook 1=%v hook 2=%v want %v", got1.ID, got2.ID, td.ID)
	}
}

func TestRegisterOnTakedownCreated_NilHookIgnored(t *testing.T) {
	marketplace.ResetOnTakedownCreatedForTest()
	marketplace.RegisterOnTakedownCreated(nil)
	if err := marketplace.DispatchOnTakedownCreated(context.Background(), marketplace.Takedown{}, ""); err != nil {
		t.Errorf("dispatch with only-nil registry should not error, got: %v", err)
	}
}

func TestRegisterOnTakedownCreated_ErrorsJoined(t *testing.T) {
	marketplace.ResetOnTakedownCreatedForTest()
	e1 := errors.New("first")
	e2 := errors.New("second")
	var calls int
	marketplace.RegisterOnTakedownCreated(func(_ context.Context, _ marketplace.Takedown, _ string) error {
		calls++
		return e1
	})
	marketplace.RegisterOnTakedownCreated(func(_ context.Context, _ marketplace.Takedown, _ string) error {
		calls++
		return e2
	})
	err := marketplace.DispatchOnTakedownCreated(context.Background(), marketplace.Takedown{}, "")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, e1) || !errors.Is(err, e2) {
		t.Errorf("want joined errors containing both, got %v", err)
	}
	if calls != 2 {
		t.Errorf("want both hooks invoked, got %d calls", calls)
	}
}

func TestNewDerivativeNotifier_NilNotifierPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil notifier")
		}
	}()
	_ = marketplace.NewDerivativeNotifier(nil, nil)
}

// contains is a tiny dependency-free substring check so the body tests
// don't pull in strings.Contains via the import block (already used
// elsewhere in the package but kept local here to be explicit about
// intent).
func contains(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

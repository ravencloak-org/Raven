package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/internal/service"
)

// fakeConvRepo is an in-memory ConversationRepositoryI implementation.
type fakeConvRepo struct {
	mu       sync.Mutex
	byID     map[string]*model.ConversationSession
	byUserKB map[string][]*model.ConversationSession
}

func newFakeConvRepo() *fakeConvRepo {
	return &fakeConvRepo{
		byID:     map[string]*model.ConversationSession{},
		byUserKB: map[string][]*model.ConversationSession{},
	}
}

func key(org, kb, user string) string { return org + "|" + kb + "|" + user }

func (f *fakeConvRepo) CreateSession(ctx context.Context, orgID, kbID, userID, channel string, initial []model.ConversationTurn) (*model.ConversationSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := &model.ConversationSession{
		ID:        "sess-" + orgID + "-" + userID + "-" + time.Now().Format("150405.000000"),
		OrgID:     orgID,
		KBID:      kbID,
		UserID:    userID,
		Channel:   channel,
		Messages:  append([]model.ConversationTurn{}, initial...),
		StartedAt: time.Now().UTC(),
	}
	f.byID[s.ID] = s
	f.byUserKB[key(orgID, kbID, userID)] = append(f.byUserKB[key(orgID, kbID, userID)], s)
	return s, nil
}

func (f *fakeConvRepo) AppendMessage(ctx context.Context, orgID, sessionID string, turn model.ConversationTurn) (*model.ConversationSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[sessionID]
	if !ok || s.OrgID != orgID {
		return nil, repository.ErrConversationNotFound
	}
	s.Messages = append(s.Messages, turn)
	return s, nil
}

func (f *fakeConvRepo) EndSession(ctx context.Context, orgID, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[sessionID]
	if !ok || s.OrgID != orgID {
		return repository.ErrConversationNotFound
	}
	if s.EndedAt == nil {
		t := time.Now().UTC()
		s.EndedAt = &t
	}
	return nil
}

func (f *fakeConvRepo) GetRecentByUser(ctx context.Context, orgID, kbID, userID string, limit int) ([]model.ConversationSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := f.byUserKB[key(orgID, kbID, userID)]
	// Return newest-first.
	out := make([]model.ConversationSession, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		out = append(out, *list[i])
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *fakeConvRepo) GetByID(ctx context.Context, orgID, sessionID, userID string) (*model.ConversationSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[sessionID]
	if !ok || s.OrgID != orgID || s.UserID != userID {
		return nil, repository.ErrConversationNotFound
	}
	cp := *s
	return &cp, nil
}

func (f *fakeConvRepo) ListByUser(ctx context.Context, orgID, kbID, userID string, limit, offset int) ([]model.ConversationSessionSummary, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	list := f.byUserKB[key(orgID, kbID, userID)]
	total := len(list)
	out := make([]model.ConversationSessionSummary, 0, len(list))
	// Newest-first.
	for i := len(list) - 1; i >= 0; i-- {
		s := list[i]
		out = append(out, model.ConversationSessionSummary{
			ID: s.ID, OrgID: s.OrgID, KBID: s.KBID, UserID: s.UserID,
			Channel: s.Channel, StartedAt: s.StartedAt, EndedAt: s.EndedAt,
			MessageCount: len(s.Messages), Summary: s.Summary,
		})
	}
	if offset > len(out) {
		return nil, total, nil
	}
	out = out[offset:]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, total, nil
}

// fakeAnalytics captures Capture calls for assertions.
type fakeAnalytics struct {
	mu     sync.Mutex
	events []capturedEvent
	fail   error
}
type capturedEvent struct {
	distinctID string
	event      string
	props      map[string]any
}

func (a *fakeAnalytics) Capture(ctx context.Context, distinctID, event string, props map[string]any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, capturedEvent{distinctID: distinctID, event: event, props: props})
	return a.fail
}

func (a *fakeAnalytics) wait(n int) []capturedEvent {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		if len(a.events) >= n {
			out := make([]capturedEvent, len(a.events))
			copy(out, a.events)
			a.mu.Unlock()
			return out
		}
		a.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]capturedEvent, len(a.events))
	copy(out, a.events)
	return out
}

func TestConversationService_StartSession_EmitsSessionStarted(t *testing.T) {
	repo := newFakeConvRepo()
	an := &fakeAnalytics{}
	svc := service.NewConversationService(repo, an)

	sess, err := svc.StartSession(context.Background(), "org-1", "kb-1", "user-42", model.ConvChannelChat)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	if sess.UserID != "user-42" || sess.OrgID != "org-1" || sess.KBID != "kb-1" {
		t.Fatalf("session fields wrong: %+v", sess)
	}
	events := an.wait(1)
	if len(events) != 1 || events[0].event != "session_started" {
		t.Fatalf("expected 1 session_started event, got %+v", events)
	}
	if events[0].distinctID != "user-42" {
		t.Errorf("distinct_id should be JWT sub (user-42), got %q", events[0].distinctID)
	}
	if events[0].props["kb_id"] != "kb-1" {
		t.Errorf("kb_id missing in event props")
	}
}

func TestConversationService_RecordMessage_UserRoleEmitsMessageSent(t *testing.T) {
	repo := newFakeConvRepo()
	an := &fakeAnalytics{}
	svc := service.NewConversationService(repo, an)

	sess, err := svc.StartSession(context.Background(), "org-1", "kb-1", "user-42", model.ConvChannelChat)
	if err != nil {
		t.Fatal(err)
	}
	_ = an.wait(1) // session_started

	_, err = svc.RecordMessage(context.Background(), "org-1", sess.ID, "user-42", "kb-1", model.ConvChannelChat, "user", "hello", true, 3)
	if err != nil {
		t.Fatalf("RecordMessage: %v", err)
	}
	events := an.wait(2)
	if len(events) < 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	msgEvent := events[1]
	if msgEvent.event != "message_sent" {
		t.Fatalf("want message_sent, got %q", msgEvent.event)
	}
	if msgEvent.props["is_returning_user"] != true {
		t.Errorf("is_returning_user should be true")
	}
	if msgEvent.props["history_turns_injected"] != 3 {
		t.Errorf("history_turns_injected want 3 got %v", msgEvent.props["history_turns_injected"])
	}
}

func TestConversationService_RecordMessage_AssistantDoesNotEmitMessageSent(t *testing.T) {
	repo := newFakeConvRepo()
	an := &fakeAnalytics{}
	svc := service.NewConversationService(repo, an)

	sess, err := svc.StartSession(context.Background(), "org-1", "kb-1", "user-1", model.ConvChannelChat)
	if err != nil {
		t.Fatal(err)
	}
	_ = an.wait(1)
	_, err = svc.RecordMessage(context.Background(), "org-1", sess.ID, "user-1", "kb-1", model.ConvChannelChat, "assistant", "hi there", false, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Give any spurious event a moment.
	time.Sleep(50 * time.Millisecond)
	an.mu.Lock()
	defer an.mu.Unlock()
	for _, e := range an.events[1:] {
		if e.event == "message_sent" {
			t.Fatalf("assistant turn must not emit message_sent")
		}
	}
}

func TestConversationService_RecentHistory_CapsToMaxTurns(t *testing.T) {
	repo := newFakeConvRepo()
	svc := service.NewConversationService(repo, nil)
	ctx := context.Background()

	sess, err := svc.StartSession(ctx, "org-1", "kb-1", "user-42", model.ConvChannelChat)
	if err != nil {
		t.Fatal(err)
	}
	// Seed 12 turns.
	for i := 0; i < 12; i++ {
		if _, err := svc.RecordMessage(ctx, "org-1", sess.ID, "user-42", "kb-1", model.ConvChannelChat, "user", "m", false, 0); err != nil {
			t.Fatal(err)
		}
	}
	hist, err := svc.RecentHistory(ctx, "org-1", "kb-1", "user-42", 0)
	if err != nil {
		t.Fatalf("RecentHistory: %v", err)
	}
	if len(hist) != model.MaxConversationHistoryTurns {
		t.Fatalf("expected %d turns, got %d", model.MaxConversationHistoryTurns, len(hist))
	}
}

func TestConversationService_IsReturningUser(t *testing.T) {
	repo := newFakeConvRepo()
	svc := service.NewConversationService(repo, nil)
	ctx := context.Background()

	ret, err := svc.IsReturningUser(ctx, "org-1", "kb-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if ret {
		t.Fatal("new user must not be returning")
	}
	if _, err := svc.StartSession(ctx, "org-1", "kb-1", "user-1", model.ConvChannelChat); err != nil {
		t.Fatal(err)
	}
	ret, err = svc.IsReturningUser(ctx, "org-1", "kb-1", "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if !ret {
		t.Fatal("user with prior session must be returning")
	}
}

func TestConversationService_GetTranscript_CrossUserIsolation(t *testing.T) {
	repo := newFakeConvRepo()
	svc := service.NewConversationService(repo, nil)
	ctx := context.Background()

	sess, err := svc.StartSession(ctx, "org-1", "kb-1", "alice", model.ConvChannelChat)
	if err != nil {
		t.Fatal(err)
	}
	// Bob must not be able to fetch Alice's transcript.
	_, err = svc.GetTranscript(ctx, "org-1", "kb-1", sess.ID, "bob")
	if err == nil {
		t.Fatal("expected not-found for cross-user transcript fetch")
	}
}

func TestConversationService_RecordMessage_NotFound(t *testing.T) {
	repo := newFakeConvRepo()
	svc := service.NewConversationService(repo, nil)
	_, err := svc.RecordMessage(context.Background(), "org-1", "nope", "u", "kb", "chat", "user", "x", false, 0)
	if err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestConversationService_CaptureSwallowsAnalyticsError(t *testing.T) {
	repo := newFakeConvRepo()
	an := &fakeAnalytics{fail: errors.New("posthog down")}
	svc := service.NewConversationService(repo, an)

	_, err := svc.StartSession(context.Background(), "org-1", "kb-1", "user-1", model.ConvChannelChat)
	if err != nil {
		t.Fatalf("StartSession must not fail when PostHog errors: %v", err)
	}
}

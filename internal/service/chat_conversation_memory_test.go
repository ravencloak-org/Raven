package service

import (
	"context"
	"sync"
	"testing"

	"github.com/ravencloak-org/Raven/internal/model"
)

// spyConvMem records every interaction with the ConversationMemory interface
// so assertions can verify both user + assistant turns are appended.
type spyConvMem struct {
	mu              sync.Mutex
	startSessions   int
	recorded        []spyRecorded
	sessIDToReturn  string
	recentHist      []model.ConversationTurn
}

type spyRecorded struct {
	sessionID string
	role      string
	content   string
}

func (s *spyConvMem) RecentHistory(ctx context.Context, orgID, kbID, userID string, maxTurns int) ([]model.ConversationTurn, error) {
	return s.recentHist, nil
}

func (s *spyConvMem) IsReturningUser(ctx context.Context, orgID, kbID, userID string) (bool, error) {
	return len(s.recentHist) > 0, nil
}

func (s *spyConvMem) RecordMessage(ctx context.Context, orgID, sessionID, userID, kbID, channel, role, content string, isReturningUser bool, historyTurnsInjected int) (*model.ConversationSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorded = append(s.recorded, spyRecorded{sessionID: sessionID, role: role, content: content})
	return &model.ConversationSession{ID: sessionID, OrgID: orgID, KBID: kbID, UserID: userID, Channel: channel}, nil
}

func (s *spyConvMem) StartSession(ctx context.Context, orgID, kbID, userID, channel string) (*model.ConversationSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startSessions++
	return &model.ConversationSession{ID: s.sessIDToReturn, OrgID: orgID, KBID: kbID, UserID: userID, Channel: channel}, nil
}

// TestPrimeConversationMemory_LazilyStartsSessionAndAppendsUserTurn is the
// regression for PR #349 review thread #1: the chat path must drive
// RecordMessage for the user turn (and an assistant turn must follow), and
// when no ConversationSessionID is supplied a fresh session is created.
func TestPrimeConversationMemory_LazilyStartsSessionAndAppendsUserTurn(t *testing.T) {
	spy := &spyConvMem{sessIDToReturn: "new-sess-1"}
	svc := (&ChatService{}).WithConversationMemory(spy)

	filters := map[string]string{}
	req := &model.ChatCompletionRequest{
		Query:  "hello",
		UserID: "user-1",
	}

	sessID, hInjected, isReturning := svc.primeConversationMemory(
		context.Background(), "org-1", "kb-1", req, filters,
	)

	if spy.startSessions != 1 {
		t.Fatalf("StartSession must be called exactly once when no session supplied; got %d", spy.startSessions)
	}
	if sessID != "new-sess-1" {
		t.Fatalf("session id must come from StartSession; got %q", sessID)
	}
	if hInjected != 0 || isReturning {
		t.Errorf("no history -> new user; got injected=%d returning=%v", hInjected, isReturning)
	}

	// Exactly one user-turn record against the freshly-created session.
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.recorded) != 1 {
		t.Fatalf("want 1 RecordMessage call, got %d", len(spy.recorded))
	}
	if spy.recorded[0].role != "user" || spy.recorded[0].sessionID != "new-sess-1" || spy.recorded[0].content != "hello" {
		t.Errorf("bad user turn: %+v", spy.recorded[0])
	}
}

// TestPrimeConversationMemory_ReusesCallerSessionID verifies that an
// already-provided conversation_session_id is NOT overwritten by a fresh
// StartSession (voice flow pre-creates it).
func TestPrimeConversationMemory_ReusesCallerSessionID(t *testing.T) {
	spy := &spyConvMem{sessIDToReturn: "should-not-be-used"}
	svc := (&ChatService{}).WithConversationMemory(spy)

	req := &model.ChatCompletionRequest{
		Query:                 "hi",
		UserID:                "user-1",
		ConversationSessionID: "caller-sess-9",
	}
	sessID, _, _ := svc.primeConversationMemory(context.Background(), "org-1", "kb-1", req, map[string]string{})

	if sessID != "caller-sess-9" {
		t.Fatalf("caller-provided session must be preserved, got %q", sessID)
	}
	if spy.startSessions != 0 {
		t.Fatalf("StartSession must not be called when caller provides a session; got %d", spy.startSessions)
	}
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.recorded) != 1 || spy.recorded[0].sessionID != "caller-sess-9" {
		t.Fatalf("user turn must record against caller session; got %+v", spy.recorded)
	}
}

// TestRecordAssistantMemoryTurn_AppendsAssistantRole closes the loop from
// review thread #1: the assistant reply must also be persisted so the
// transcript is complete for cross-channel recall.
func TestRecordAssistantMemoryTurn_AppendsAssistantRole(t *testing.T) {
	spy := &spyConvMem{}
	svc := (&ChatService{}).WithConversationMemory(spy)

	svc.recordAssistantMemoryTurn(context.Background(), "org-1", "kb-1", "user-1", "sess-1", "the answer", false, 0)

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.recorded) != 1 {
		t.Fatalf("want 1 RecordMessage call, got %d", len(spy.recorded))
	}
	got := spy.recorded[0]
	if got.role != "assistant" || got.content != "the answer" || got.sessionID != "sess-1" {
		t.Errorf("bad assistant turn: %+v", got)
	}
}

// TestFullChatLifecycle_PersistsBothRoles verifies the exact regression
// described in PR #349 review thread #1: a no-op memory feature had to
// transition to a state where both user AND assistant turns are persisted.
func TestFullChatLifecycle_PersistsBothRoles(t *testing.T) {
	spy := &spyConvMem{sessIDToReturn: "lifecycle-sess"}
	svc := (&ChatService{}).WithConversationMemory(spy)

	req := &model.ChatCompletionRequest{Query: "user-question", UserID: "user-42"}

	convSessionID, hInjected, isReturning := svc.primeConversationMemory(context.Background(), "org-1", "kb-1", req, map[string]string{})
	svc.recordAssistantMemoryTurn(context.Background(), "org-1", "kb-1", req.UserID, convSessionID, "assistant-reply", isReturning, hInjected)

	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.recorded) != 2 {
		t.Fatalf("expected user+assistant turns (2), got %d: %+v", len(spy.recorded), spy.recorded)
	}
	if spy.recorded[0].role != "user" || spy.recorded[1].role != "assistant" {
		t.Errorf("turn order wrong: %+v", spy.recorded)
	}
	// Critically: both turns bind to the SAME session id.
	if spy.recorded[0].sessionID != spy.recorded[1].sessionID {
		t.Errorf("both turns must share session id, got user=%q assistant=%q",
			spy.recorded[0].sessionID, spy.recorded[1].sessionID)
	}
}

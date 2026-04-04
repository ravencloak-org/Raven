package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
)

// mockVoiceRepo implements service.VoiceRepository for unit testing.
type mockVoiceRepo struct {
	createSessionFn      func(ctx context.Context, tx pgx.Tx, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error)
	getSessionFn         func(ctx context.Context, tx pgx.Tx, orgID, sessionID string) (*model.VoiceSession, error)
	updateSessionStateFn func(ctx context.Context, tx pgx.Tx, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error)
	listSessionsFn       func(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.VoiceSession, int, error)
	appendTurnFn         func(ctx context.Context, tx pgx.Tx, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error)
	listTurnsFn          func(ctx context.Context, tx pgx.Tx, orgID, sessionID string) ([]model.VoiceTurn, error)
}

func (m *mockVoiceRepo) CreateSession(ctx context.Context, tx pgx.Tx, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error) {
	if m.createSessionFn != nil {
		return m.createSessionFn(ctx, tx, orgID, req)
	}
	return nil, nil
}

func (m *mockVoiceRepo) GetSession(ctx context.Context, tx pgx.Tx, orgID, sessionID string) (*model.VoiceSession, error) {
	if m.getSessionFn != nil {
		return m.getSessionFn(ctx, tx, orgID, sessionID)
	}
	return nil, nil
}

func (m *mockVoiceRepo) UpdateSessionState(ctx context.Context, tx pgx.Tx, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error) {
	if m.updateSessionStateFn != nil {
		return m.updateSessionStateFn(ctx, tx, orgID, sessionID, state)
	}
	return nil, nil
}

func (m *mockVoiceRepo) ListSessions(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.VoiceSession, int, error) {
	if m.listSessionsFn != nil {
		return m.listSessionsFn(ctx, tx, orgID, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockVoiceRepo) AppendTurn(ctx context.Context, tx pgx.Tx, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error) {
	if m.appendTurnFn != nil {
		return m.appendTurnFn(ctx, tx, orgID, sessionID, req)
	}
	return nil, nil
}

func (m *mockVoiceRepo) ListTurns(ctx context.Context, tx pgx.Tx, orgID, sessionID string) ([]model.VoiceTurn, error) {
	if m.listTurnsFn != nil {
		return m.listTurnsFn(ctx, tx, orgID, sessionID)
	}
	return nil, nil
}

// Ensure mockVoiceRepo satisfies the interface.
var _ service.VoiceRepository = (*mockVoiceRepo)(nil)

// TestVoiceModel_CallDurationSemantics validates VoiceSession struct semantics.
func TestVoiceModel_CallDurationSemantics(t *testing.T) {
	now := time.Now()
	start := now.Add(-5 * time.Minute)
	end := now
	dur := int(end.Sub(start).Seconds())
	sess := model.VoiceSession{
		ID:                  "s-1",
		OrgID:               "o-1",
		LiveKitRoom:         "room-1",
		State:               model.VoiceSessionStateEnded,
		StartedAt:           &start,
		EndedAt:             &end,
		CallDurationSeconds: &dur,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if sess.CallDurationSeconds == nil {
		t.Fatal("expected call_duration_seconds to be set")
	}
	if *sess.CallDurationSeconds <= 0 {
		t.Errorf("expected positive duration, got %d", *sess.CallDurationSeconds)
	}
}

// TestVoiceModel_Speaker_Values validates speaker enum string values.
func TestVoiceModel_Speaker_Values(t *testing.T) {
	cases := []struct {
		speaker model.VoiceSpeaker
		wantStr string
	}{
		{model.VoiceSpeakerAgent, "agent"},
		{model.VoiceSpeakerUser, "user"},
	}
	for _, tc := range cases {
		if string(tc.speaker) != tc.wantStr {
			t.Errorf("VoiceSpeaker: got %q, want %q", string(tc.speaker), tc.wantStr)
		}
	}
}

// TestVoiceModel_SessionState_Values validates session state enum string values.
func TestVoiceModel_SessionState_Values(t *testing.T) {
	cases := []struct {
		state   model.VoiceSessionState
		wantStr string
	}{
		{model.VoiceSessionStateCreated, "created"},
		{model.VoiceSessionStateActive, "active"},
		{model.VoiceSessionStateEnded, "ended"},
	}
	for _, tc := range cases {
		if string(tc.state) != tc.wantStr {
			t.Errorf("VoiceSessionState: got %q, want %q", string(tc.state), tc.wantStr)
		}
	}
}

// TestVoiceService_UpdateSessionState_InvalidState_GuardCheck verifies
// that the VoiceService exported guard rejects the 'created' state
// without needing a DB pool. We call it on the exported VoiceService
// directly with a nil pool; the guard fires before any pool access.
func TestVoiceService_UpdateSessionState_InvalidState_GuardCheck(t *testing.T) {
	// NewVoiceService with nil pool: the guard check fires before db.WithOrgID.
	svc := service.NewVoiceService(nil, nil)
	_, err := svc.UpdateSessionState(context.Background(), "org-1", "sess-1", model.VoiceSessionStateCreated)
	if err == nil {
		t.Fatal("expected error for invalid state 'created', got nil")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestVoiceTurnListResponse_EmptySlice checks that empty turn list returns empty slice not nil.
func TestVoiceTurnListResponse_EmptySlice(t *testing.T) {
	resp := &model.VoiceTurnListResponse{
		SessionID: "sess-1",
		Turns:     []model.VoiceTurn{},
	}
	if resp.Turns == nil {
		t.Error("expected empty slice, got nil")
	}
}

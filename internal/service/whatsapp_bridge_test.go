package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
)

// mockWhatsAppBridgeRepo implements service.WhatsAppBridgeRepository.
type mockWhatsAppBridgeRepo struct {
	createBridgeFn     func(ctx context.Context, tx pgx.Tx, b *model.WhatsAppBridge) (*model.WhatsAppBridge, error)
	getByCallIDFn      func(ctx context.Context, tx pgx.Tx, orgID, callID string) (*model.WhatsAppBridge, error)
	getByIDFn          func(ctx context.Context, tx pgx.Tx, orgID, bridgeID string) (*model.WhatsAppBridge, error)
	updateStateFn      func(ctx context.Context, tx pgx.Tx, orgID, callID string, state model.BridgeState) (*model.WhatsAppBridge, error)
	updateSDPAnswerFn  func(ctx context.Context, tx pgx.Tx, orgID, callID, sdpAnswer string) (*model.WhatsAppBridge, error)
	listActiveFn       func(ctx context.Context, tx pgx.Tx, orgID string) ([]model.WhatsAppBridge, error)
}

func (m *mockWhatsAppBridgeRepo) CreateBridge(ctx context.Context, tx pgx.Tx, b *model.WhatsAppBridge) (*model.WhatsAppBridge, error) {
	if m.createBridgeFn != nil {
		return m.createBridgeFn(ctx, tx, b)
	}
	return nil, nil
}

func (m *mockWhatsAppBridgeRepo) GetByCallID(ctx context.Context, tx pgx.Tx, orgID, callID string) (*model.WhatsAppBridge, error) {
	if m.getByCallIDFn != nil {
		return m.getByCallIDFn(ctx, tx, orgID, callID)
	}
	return nil, nil
}

func (m *mockWhatsAppBridgeRepo) GetByID(ctx context.Context, tx pgx.Tx, orgID, bridgeID string) (*model.WhatsAppBridge, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, tx, orgID, bridgeID)
	}
	return nil, nil
}

func (m *mockWhatsAppBridgeRepo) UpdateState(ctx context.Context, tx pgx.Tx, orgID, callID string, state model.BridgeState) (*model.WhatsAppBridge, error) {
	if m.updateStateFn != nil {
		return m.updateStateFn(ctx, tx, orgID, callID, state)
	}
	return nil, nil
}

func (m *mockWhatsAppBridgeRepo) UpdateSDPAnswer(ctx context.Context, tx pgx.Tx, orgID, callID, sdpAnswer string) (*model.WhatsAppBridge, error) {
	if m.updateSDPAnswerFn != nil {
		return m.updateSDPAnswerFn(ctx, tx, orgID, callID, sdpAnswer)
	}
	return nil, nil
}

func (m *mockWhatsAppBridgeRepo) ListActive(ctx context.Context, tx pgx.Tx, orgID string) ([]model.WhatsAppBridge, error) {
	if m.listActiveFn != nil {
		return m.listActiveFn(ctx, tx, orgID)
	}
	return nil, nil
}

var _ service.WhatsAppBridgeRepository = (*mockWhatsAppBridgeRepo)(nil)

// mockSDPRelay implements service.SDPRelay.
type mockSDPRelay struct {
	generateAnswerFn func(ctx context.Context, roomName, sdpOffer string) (string, error)
}

func (m *mockSDPRelay) GenerateAnswer(ctx context.Context, roomName, sdpOffer string) (string, error) {
	if m.generateAnswerFn != nil {
		return m.generateAnswerFn(ctx, roomName, sdpOffer)
	}
	return "v=0\r\no=raven 0 0 IN IP4 0.0.0.0\r\ns=mock\r\n", nil
}

var _ service.SDPRelay = (*mockSDPRelay)(nil)

// mockLiveKitClient is defined in voice_test.go and shared across service_test package.

// --- CreateBridge service-level tests (no DB pool needed for validation) ---

func TestWhatsAppBridgeService_CreateBridge_NilRequest(t *testing.T) {
	svc := service.NewWhatsAppBridgeService(nil, nil, nil, nil, nil)
	_, err := svc.CreateBridge(context.Background(), "org-1", "call-1", nil)
	if err == nil {
		t.Fatal("expected error for nil request, got nil")
	}
}

func TestWhatsAppBridgeService_CreateBridge_EmptySDP(t *testing.T) {
	svc := service.NewWhatsAppBridgeService(nil, nil, nil, nil, nil)
	_, err := svc.CreateBridge(context.Background(), "org-1", "call-1", &model.CreateBridgeRequest{
		SDPOffer: "",
	})
	if err == nil {
		t.Fatal("expected error for empty SDP offer, got nil")
	}
}

func TestWhatsAppBridgeService_CreateBridge_LiveKitError(t *testing.T) {
	lk := &mockLiveKitClient{
		createRoomFn: func(_ context.Context, _, _ string) error {
			return fmt.Errorf("livekit connection refused")
		},
	}
	svc := service.NewWhatsAppBridgeService(nil, nil, nil, lk, nil)
	_, err := svc.CreateBridge(context.Background(), "org-12345678", "call-1", &model.CreateBridgeRequest{
		SDPOffer: "v=0\r\n",
	})
	if err == nil {
		t.Fatal("expected error when LiveKit fails, got nil")
	}
}

func TestWhatsAppBridgeService_CreateBridge_SDPRelayError(t *testing.T) {
	lk := &mockLiveKitClient{}
	relay := &mockSDPRelay{
		generateAnswerFn: func(_ context.Context, _, _ string) (string, error) {
			return "", fmt.Errorf("SDP negotiation failed")
		},
	}
	svc := service.NewWhatsAppBridgeService(nil, nil, nil, lk, relay)
	_, err := svc.CreateBridge(context.Background(), "org-12345678", "call-1", &model.CreateBridgeRequest{
		SDPOffer: "v=0\r\n",
	})
	if err == nil {
		t.Fatal("expected error when SDP relay fails, got nil")
	}
}

// --- Model tests ---

func TestBridgeState_Values(t *testing.T) {
	cases := []struct {
		state   model.BridgeState
		wantStr string
	}{
		{model.BridgeStateInitializing, "initializing"},
		{model.BridgeStateActive, "active"},
		{model.BridgeStateFailed, "failed"},
		{model.BridgeStateClosed, "closed"},
	}
	for _, tc := range cases {
		if string(tc.state) != tc.wantStr {
			t.Errorf("BridgeState: got %q, want %q", string(tc.state), tc.wantStr)
		}
	}
}

func TestWhatsAppBridge_VoiceSessionID_Optional(t *testing.T) {
	b := model.WhatsAppBridge{
		ID:          "bridge-1",
		OrgID:       "org-1",
		CallID:      "call-1",
		LiveKitRoom: "room-1",
		BridgeState: model.BridgeStateActive,
	}
	if b.VoiceSessionID != nil {
		t.Error("expected nil voice_session_id")
	}

	sessionID := "session-1"
	b.VoiceSessionID = &sessionID
	if b.VoiceSessionID == nil || *b.VoiceSessionID != "session-1" {
		t.Error("expected voice_session_id to be set")
	}
}

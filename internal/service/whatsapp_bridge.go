package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
	livekitpkg "github.com/ravencloak-org/Raven/pkg/livekit"
)

// WhatsAppBridgeRepository defines the persistence interface for bridge operations.
type WhatsAppBridgeRepository interface {
	CreateBridge(ctx context.Context, tx pgx.Tx, b *model.WhatsAppBridge) (*model.WhatsAppBridge, error)
	GetByCallID(ctx context.Context, tx pgx.Tx, orgID, callID string) (*model.WhatsAppBridge, error)
	GetByID(ctx context.Context, tx pgx.Tx, orgID, bridgeID string) (*model.WhatsAppBridge, error)
	UpdateState(ctx context.Context, tx pgx.Tx, orgID, callID string, state model.BridgeState) (*model.WhatsAppBridge, error)
	UpdateSDPAnswer(ctx context.Context, tx pgx.Tx, orgID, callID, sdpAnswer string) (*model.WhatsAppBridge, error)
	ListActive(ctx context.Context, tx pgx.Tx, orgID string) ([]model.WhatsAppBridge, error)
}

// SDPRelay handles SDP offer/answer negotiation for bridging WebRTC streams.
type SDPRelay interface {
	// GenerateAnswer takes an SDP offer from Meta's WebRTC stack and produces
	// an SDP answer that routes audio through the LiveKit room.
	GenerateAnswer(ctx context.Context, roomName, sdpOffer string) (string, error)
}

// WhatsAppBridgeService contains business logic for bridging WhatsApp calls to LiveKit rooms.
type WhatsAppBridgeService struct {
	repo      WhatsAppBridgeRepository
	voiceRepo VoiceRepository
	pool      *pgxpool.Pool
	lkClient  livekitpkg.RoomClient
	sdpRelay  SDPRelay
}

// NewWhatsAppBridgeService creates a new WhatsAppBridgeService.
func NewWhatsAppBridgeService(
	repo WhatsAppBridgeRepository,
	voiceRepo VoiceRepository,
	pool *pgxpool.Pool,
	lkClient livekitpkg.RoomClient,
	sdpRelay SDPRelay,
) *WhatsAppBridgeService {
	return &WhatsAppBridgeService{
		repo:      repo,
		voiceRepo: voiceRepo,
		pool:      pool,
		lkClient:  lkClient,
		sdpRelay:  sdpRelay,
	}
}

// bridgeMetadata is the JSON metadata attached to the LiveKit room.
type bridgeMetadata struct {
	Source      string `json:"source"`
	CallID      string `json:"call_id"`
	OrgID       string `json:"org_id"`
	AutoBridged bool   `json:"auto_bridged"`
}

// CreateBridge creates a LiveKit room, generates an SDP answer, and records
// the bridge in the database. The voice agent auto-joins the room via
// LiveKit's agent dispatch mechanism.
func (s *WhatsAppBridgeService) CreateBridge(ctx context.Context, orgID, callID string, req *model.CreateBridgeRequest) (*model.CreateBridgeResponse, error) {
	if req == nil {
		return nil, apierror.NewBadRequest("request body must not be nil")
	}
	if req.SDPOffer == "" {
		return nil, apierror.NewBadRequest("sdp_offer is required")
	}

	// Generate a unique room name for this WhatsApp call.
	roomName := fmt.Sprintf("wa-%s-%s", orgID[:8], callID)

	// Build room metadata so the voice agent knows this is a WhatsApp call.
	meta := bridgeMetadata{
		Source:      "whatsapp",
		CallID:      callID,
		OrgID:       orgID,
		AutoBridged: req.AutoBridge,
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		slog.ErrorContext(ctx, "WhatsAppBridgeService.CreateBridge marshal metadata", "error", err)
		return nil, apierror.NewInternal("failed to marshal bridge metadata")
	}
	metaStr := string(metaBytes)

	// Create the LiveKit room (auto-creates on first token join, but we call
	// CreateRoom for explicit room setup with metadata).
	if err := s.lkClient.CreateRoom(ctx, roomName, metaStr); err != nil {
		slog.ErrorContext(ctx, "WhatsAppBridgeService.CreateBridge livekit create room", "error", err)
		return nil, apierror.NewInternal("failed to create LiveKit room")
	}

	// Generate SDP answer that bridges WhatsApp audio to the LiveKit room.
	sdpAnswer, err := s.sdpRelay.GenerateAnswer(ctx, roomName, req.SDPOffer)
	if err != nil {
		slog.ErrorContext(ctx, "WhatsAppBridgeService.CreateBridge SDP relay", "error", err)
		return nil, apierror.NewInternal("failed to generate SDP answer")
	}

	var bridge *model.WhatsAppBridge
	err = db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Create a voice session for tracking this call.
		voiceSession, e := s.voiceRepo.CreateSession(ctx, tx, orgID, &model.CreateVoiceSessionRequest{
			LiveKitRoom: roomName,
		})
		if e != nil {
			return fmt.Errorf("create voice session: %w", e)
		}

		// Persist the bridge record.
		bridge, e = s.repo.CreateBridge(ctx, tx, &model.WhatsAppBridge{
			OrgID:          orgID,
			CallID:         callID,
			LiveKitRoom:    roomName,
			BridgeState:    model.BridgeStateActive,
			VoiceSessionID: &voiceSession.ID,
			SDPOffer:       req.SDPOffer,
			SDPAnswer:      sdpAnswer,
			Metadata:       metaStr,
		})
		if e != nil {
			return fmt.Errorf("create bridge: %w", e)
		}

		// Activate the voice session.
		_, e = s.voiceRepo.UpdateSessionState(ctx, tx, orgID, voiceSession.ID, model.VoiceSessionStateActive)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "WhatsAppBridgeService.CreateBridge db error", "error", err)
		return nil, apierror.NewInternal("failed to create bridge")
	}

	return &model.CreateBridgeResponse{
		Bridge:    *bridge,
		SDPAnswer: sdpAnswer,
	}, nil
}

// GetBridge retrieves the bridge for a specific call.
func (s *WhatsAppBridgeService) GetBridge(ctx context.Context, orgID, callID string) (*model.WhatsAppBridge, error) {
	var bridge *model.WhatsAppBridge
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		bridge, e = s.repo.GetByCallID(ctx, tx, orgID, callID)
		return e
	})
	if err != nil {
		if isBridgeNotFound(err) {
			return nil, apierror.NewNotFound("bridge not found")
		}
		slog.ErrorContext(ctx, "WhatsAppBridgeService.GetBridge db error", "error", err)
		return nil, apierror.NewInternal("failed to get bridge")
	}
	return bridge, nil
}

// TeardownBridge closes an active bridge: ends the voice session, deletes
// the LiveKit room, and marks the bridge as closed.
func (s *WhatsAppBridgeService) TeardownBridge(ctx context.Context, orgID, callID string) (*model.WhatsAppBridge, error) {
	var bridge *model.WhatsAppBridge

	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Fetch the current bridge.
		b, e := s.repo.GetByCallID(ctx, tx, orgID, callID)
		if e != nil {
			return e
		}

		if b.BridgeState == model.BridgeStateClosed {
			bridge = b
			return nil // already closed, idempotent
		}

		// End the associated voice session if one exists.
		if b.VoiceSessionID != nil {
			_, _ = s.voiceRepo.UpdateSessionState(ctx, tx, orgID, *b.VoiceSessionID, model.VoiceSessionStateEnded)
		}

		// Update bridge state to closed.
		bridge, e = s.repo.UpdateState(ctx, tx, orgID, callID, model.BridgeStateClosed)
		return e
	})
	if err != nil {
		if isBridgeNotFound(err) {
			return nil, apierror.NewNotFound("bridge not found")
		}
		slog.ErrorContext(ctx, "WhatsAppBridgeService.TeardownBridge db error", "error", err)
		return nil, apierror.NewInternal("failed to teardown bridge")
	}

	// Delete the LiveKit room (best-effort, fire-and-forget).
	if bridge != nil && bridge.LiveKitRoom != "" {
		if err := s.lkClient.DeleteRoom(ctx, bridge.LiveKitRoom); err != nil {
			slog.WarnContext(ctx, "WhatsAppBridgeService.TeardownBridge livekit delete room failed", "error", err, "room", bridge.LiveKitRoom)
		}
	}

	return bridge, nil
}

// ListActiveBridges returns all active bridges for an org.
func (s *WhatsAppBridgeService) ListActiveBridges(ctx context.Context, orgID string) ([]model.WhatsAppBridge, error) {
	var bridges []model.WhatsAppBridge
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		bridges, e = s.repo.ListActive(ctx, tx, orgID)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "WhatsAppBridgeService.ListActiveBridges db error", "error", err)
		return nil, apierror.NewInternal("failed to list active bridges")
	}
	if bridges == nil {
		bridges = []model.WhatsAppBridge{}
	}
	return bridges, nil
}

// isBridgeNotFound checks if an error indicates a missing record.
func isBridgeNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "no rows in result set") ||
		strings.HasSuffix(msg, "not found")
}

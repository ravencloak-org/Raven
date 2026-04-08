package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/model"
)

// WhatsAppBridgeRepository handles database operations for WhatsApp-LiveKit bridges.
type WhatsAppBridgeRepository struct {
	pool *pgxpool.Pool
}

// NewWhatsAppBridgeRepository creates a new WhatsAppBridgeRepository.
func NewWhatsAppBridgeRepository(pool *pgxpool.Pool) *WhatsAppBridgeRepository {
	return &WhatsAppBridgeRepository{pool: pool}
}

const (
	sqlBridgeInsert = `
		INSERT INTO whatsapp_bridges (org_id, call_id, livekit_room, bridge_state, voice_session_id, sdp_offer, sdp_answer, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, org_id, call_id, livekit_room, bridge_state, voice_session_id,
		          sdp_offer, sdp_answer, metadata, created_at, updated_at, closed_at`

	sqlBridgeByCallID = `
		SELECT id, org_id, call_id, livekit_room, bridge_state, voice_session_id,
		       sdp_offer, sdp_answer, metadata, created_at, updated_at, closed_at
		FROM whatsapp_bridges
		WHERE call_id = $1 AND org_id = $2`

	sqlBridgeByID = `
		SELECT id, org_id, call_id, livekit_room, bridge_state, voice_session_id,
		       sdp_offer, sdp_answer, metadata, created_at, updated_at, closed_at
		FROM whatsapp_bridges
		WHERE id = $1 AND org_id = $2`

	sqlBridgeUpdateState = `
		UPDATE whatsapp_bridges SET
			bridge_state = $3,
			closed_at    = CASE WHEN $3::text = 'closed' THEN NOW() ELSE closed_at END,
			updated_at   = NOW()
		WHERE call_id = $1 AND org_id = $2
		RETURNING id, org_id, call_id, livekit_room, bridge_state, voice_session_id,
		          sdp_offer, sdp_answer, metadata, created_at, updated_at, closed_at`

	sqlBridgeUpdateSDPAnswer = `
		UPDATE whatsapp_bridges SET
			sdp_answer = $3,
			updated_at = NOW()
		WHERE call_id = $1 AND org_id = $2
		RETURNING id, org_id, call_id, livekit_room, bridge_state, voice_session_id,
		          sdp_offer, sdp_answer, metadata, created_at, updated_at, closed_at`

	sqlBridgeListActive = `
		SELECT id, org_id, call_id, livekit_room, bridge_state, voice_session_id,
		       sdp_offer, sdp_answer, metadata, created_at, updated_at, closed_at
		FROM whatsapp_bridges
		WHERE org_id = $1 AND bridge_state IN ('initializing', 'active')
		ORDER BY created_at DESC`
)

func scanBridge(row pgx.Row) (*model.WhatsAppBridge, error) {
	var b model.WhatsAppBridge
	err := row.Scan(
		&b.ID,
		&b.OrgID,
		&b.CallID,
		&b.LiveKitRoom,
		&b.BridgeState,
		&b.VoiceSessionID,
		&b.SDPOffer,
		&b.SDPAnswer,
		&b.Metadata,
		&b.CreatedAt,
		&b.UpdatedAt,
		&b.ClosedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// CreateBridge inserts a new WhatsApp-LiveKit bridge record.
func (r *WhatsAppBridgeRepository) CreateBridge(ctx context.Context, tx pgx.Tx, b *model.WhatsAppBridge) (*model.WhatsAppBridge, error) {
	row := tx.QueryRow(ctx, sqlBridgeInsert,
		b.OrgID,
		b.CallID,
		b.LiveKitRoom,
		b.BridgeState,
		b.VoiceSessionID,
		b.SDPOffer,
		b.SDPAnswer,
		b.Metadata,
	)
	result, err := scanBridge(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppBridgeRepository.CreateBridge: %w", err)
	}
	return result, nil
}

// GetByCallID retrieves a bridge by WhatsApp call ID within an org.
func (r *WhatsAppBridgeRepository) GetByCallID(ctx context.Context, tx pgx.Tx, orgID, callID string) (*model.WhatsAppBridge, error) {
	row := tx.QueryRow(ctx, sqlBridgeByCallID, callID, orgID)
	result, err := scanBridge(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppBridgeRepository.GetByCallID: %w", err)
	}
	return result, nil
}

// GetByID retrieves a bridge by its primary key within an org.
func (r *WhatsAppBridgeRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, bridgeID string) (*model.WhatsAppBridge, error) {
	row := tx.QueryRow(ctx, sqlBridgeByID, bridgeID, orgID)
	result, err := scanBridge(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppBridgeRepository.GetByID: %w", err)
	}
	return result, nil
}

// UpdateState transitions a bridge to a new state.
func (r *WhatsAppBridgeRepository) UpdateState(ctx context.Context, tx pgx.Tx, orgID, callID string, state model.BridgeState) (*model.WhatsAppBridge, error) {
	row := tx.QueryRow(ctx, sqlBridgeUpdateState, callID, orgID, state)
	result, err := scanBridge(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppBridgeRepository.UpdateState: %w", err)
	}
	return result, nil
}

// UpdateSDPAnswer sets the SDP answer on a bridge.
func (r *WhatsAppBridgeRepository) UpdateSDPAnswer(ctx context.Context, tx pgx.Tx, orgID, callID, sdpAnswer string) (*model.WhatsAppBridge, error) {
	row := tx.QueryRow(ctx, sqlBridgeUpdateSDPAnswer, callID, orgID, sdpAnswer)
	result, err := scanBridge(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppBridgeRepository.UpdateSDPAnswer: %w", err)
	}
	return result, nil
}

// ListActive returns all active bridges for an org.
func (r *WhatsAppBridgeRepository) ListActive(ctx context.Context, tx pgx.Tx, orgID string) ([]model.WhatsAppBridge, error) {
	rows, err := tx.Query(ctx, sqlBridgeListActive, orgID)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppBridgeRepository.ListActive query: %w", err)
	}
	defer rows.Close()

	var bridges []model.WhatsAppBridge
	for rows.Next() {
		var b model.WhatsAppBridge
		if err := rows.Scan(
			&b.ID,
			&b.OrgID,
			&b.CallID,
			&b.LiveKitRoom,
			&b.BridgeState,
			&b.VoiceSessionID,
			&b.SDPOffer,
			&b.SDPAnswer,
			&b.Metadata,
			&b.CreatedAt,
			&b.UpdatedAt,
			&b.ClosedAt,
		); err != nil {
			return nil, fmt.Errorf("WhatsAppBridgeRepository.ListActive scan: %w", err)
		}
		bridges = append(bridges, b)
	}
	return bridges, rows.Err()
}

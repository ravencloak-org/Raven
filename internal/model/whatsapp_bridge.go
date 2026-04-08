package model

import "time"

// BridgeState enumerates the lifecycle states of a WhatsApp-LiveKit bridge.
type BridgeState string

// Supported bridge states.
const (
	BridgeStateInitializing BridgeState = "initializing"
	BridgeStateActive       BridgeState = "active"
	BridgeStateFailed       BridgeState = "failed"
	BridgeStateClosed       BridgeState = "closed"
)

// WhatsAppCall represents a WhatsApp voice call tracked by the platform.
// This model is created here since issues #65/#66 are in parallel worktrees.
type WhatsAppCall struct {
	ID          string     `json:"id"`
	OrgID       string     `json:"org_id"`
	CallID      string     `json:"call_id"`
	PhoneNumber string     `json:"phone_number"`
	Direction   string     `json:"direction"` // "inbound" or "outbound"
	Status      string     `json:"status"`    // "ringing", "answered", "ended"
	SDPOffer    string     `json:"sdp_offer,omitempty"`
	SDPAnswer   string     `json:"sdp_answer,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	EndedAt     *time.Time `json:"ended_at,omitempty"`
}

// WhatsAppBridge represents a bridge between a WhatsApp call and a LiveKit room.
type WhatsAppBridge struct {
	ID             string      `json:"id"`
	OrgID          string      `json:"org_id"`
	CallID         string      `json:"call_id"`
	LiveKitRoom    string      `json:"livekit_room"`
	BridgeState    BridgeState `json:"bridge_state"`
	VoiceSessionID *string     `json:"voice_session_id,omitempty"`
	SDPOffer       string      `json:"sdp_offer,omitempty"`
	SDPAnswer      string      `json:"sdp_answer,omitempty"`
	Metadata       string      `json:"metadata,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	ClosedAt       *time.Time  `json:"closed_at,omitempty"`
}

// CreateBridgeRequest is the payload for POST /whatsapp/calls/:call_id/bridge.
type CreateBridgeRequest struct {
	SDPOffer string `json:"sdp_offer" binding:"required"`
	// AutoBridge when true, the bridge is created and activated automatically.
	AutoBridge bool `json:"auto_bridge,omitempty"`
}

// CreateBridgeResponse is returned after successfully creating a bridge.
type CreateBridgeResponse struct {
	Bridge    WhatsAppBridge `json:"bridge"`
	SDPAnswer string         `json:"sdp_answer"`
}

// BridgeStatusResponse is returned by the bridge status/teardown endpoints.
type BridgeStatusResponse struct {
	Bridge WhatsAppBridge `json:"bridge"`
}

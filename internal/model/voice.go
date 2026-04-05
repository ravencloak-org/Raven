package model

import "time"

// VoiceSessionState enumerates the lifecycle states of a voice session.
type VoiceSessionState string

// Supported voice session states.
const (
	VoiceSessionStateCreated VoiceSessionState = "created"
	VoiceSessionStateActive  VoiceSessionState = "active"
	VoiceSessionStateEnded   VoiceSessionState = "ended"
)

// VoiceSpeaker identifies who produced a transcription turn.
type VoiceSpeaker string

// Supported speaker values.
const (
	VoiceSpeakerAgent VoiceSpeaker = "agent"
	VoiceSpeakerUser  VoiceSpeaker = "user"
)

// VoiceSession represents a LiveKit-backed voice call scoped to an org.
type VoiceSession struct {
	ID                  string            `json:"id"`
	OrgID               string            `json:"org_id"`
	UserID              *string           `json:"user_id,omitempty"`
	StrangerID          *string           `json:"stranger_id,omitempty"`
	LiveKitRoom         string            `json:"livekit_room"`
	State               VoiceSessionState `json:"state"`
	StartedAt           *time.Time        `json:"started_at,omitempty"`
	EndedAt             *time.Time        `json:"ended_at,omitempty"`
	CallDurationSeconds *int              `json:"call_duration_seconds,omitempty"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

// VoiceTurn represents a single transcribed utterance within a voice session.
type VoiceTurn struct {
	ID         string       `json:"id"`
	SessionID  string       `json:"session_id"`
	OrgID      string       `json:"org_id"`
	Speaker    VoiceSpeaker `json:"speaker"`
	Transcript string       `json:"transcript"`
	StartedAt  time.Time    `json:"started_at"`
	EndedAt    *time.Time   `json:"ended_at,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
}

// CreateVoiceSessionRequest is the payload for POST /voice-sessions.
type CreateVoiceSessionRequest struct {
	LiveKitRoom string  `json:"livekit_room" binding:"required"`
	UserID      *string `json:"user_id,omitempty"`
	StrangerID  *string `json:"stranger_id,omitempty"`
}

// UpdateVoiceSessionStateRequest is the payload for PATCH /voice-sessions/:id.
type UpdateVoiceSessionStateRequest struct {
	State VoiceSessionState `json:"state" binding:"required"`
}

// AppendVoiceTurnRequest is the payload for POST /voice-sessions/:id/turns.
type AppendVoiceTurnRequest struct {
	Speaker    VoiceSpeaker `json:"speaker" binding:"required"`
	Transcript string       `json:"transcript" binding:"required"`
	StartedAt  time.Time    `json:"started_at" binding:"required"`
	EndedAt    *time.Time   `json:"ended_at,omitempty"`
}

// VoiceSessionListResponse is the paginated list returned by GET /voice-sessions.
type VoiceSessionListResponse struct {
	Sessions []VoiceSession `json:"sessions"`
	Total    int            `json:"total"`
	Limit    int            `json:"limit"`
	Offset   int            `json:"offset"`
}

// VoiceTurnListResponse is the list of turns for a session.
type VoiceTurnListResponse struct {
	SessionID string      `json:"session_id"`
	Turns     []VoiceTurn `json:"turns"`
}

// Package model defines shared domain types.
//
// This file models cross-channel conversation memory — the user-centric
// history shared between chat, voice, and WebRTC sessions. The underlying
// conversation_sessions table is introduced by issue #257's migration
// (00037_conversation_sessions.sql). This PR consumes that schema.
package model

import (
	"encoding/json"
	"time"
)

// ConversationChannel enumerates the supported conversation channels.
const (
	// ConvChannelChat is the text-based chat widget channel.
	ConvChannelChat = "chat"
	// ConvChannelVoice is the PSTN / voice telephony channel.
	ConvChannelVoice = "voice"
	// ConvChannelWebRTC is the browser WebRTC voice/video channel.
	ConvChannelWebRTC = "webrtc"
)

// MaxConversationHistoryTurns is the maximum number of turns surfaced to
// the AI worker on a new message. Five keeps the prompt overhead under
// roughly 1500 tokens.
const MaxConversationHistoryTurns = 5

// DefaultConversationListLimit is the default page size for conversation
// session list endpoints (distinct from MaxConversationHistoryTurns which
// bounds turns, not sessions).
const DefaultConversationListLimit = 20

// ConversationTurn is a single user or assistant message persisted inside
// a ConversationSession.messages JSONB column.
type ConversationTurn struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"ts"`
}

// ConversationSession is the cross-channel session keyed by the stable
// authenticated user ID (JWT sub claim).
type ConversationSession struct {
	ID        string             `json:"id"`
	OrgID     string             `json:"org_id"`
	KBID      string             `json:"kb_id"`
	UserID    string             `json:"user_id"`
	Channel   string             `json:"channel"`
	Messages  []ConversationTurn `json:"messages"`
	StartedAt time.Time          `json:"started_at"`
	EndedAt   *time.Time         `json:"ended_at,omitempty"`
	Summary   *string            `json:"summary,omitempty"`
}

// ConversationSessionSummary is the lightweight row returned by list endpoints.
type ConversationSessionSummary struct {
	ID           string     `json:"id"`
	OrgID        string     `json:"org_id"`
	KBID         string     `json:"kb_id"`
	UserID       string     `json:"user_id"`
	Channel      string     `json:"channel"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	MessageCount int        `json:"message_count"`
	Summary      *string    `json:"summary,omitempty"`
}

// ConversationListResponse is the paginated response for GET /conversations.
type ConversationListResponse struct {
	Sessions []ConversationSessionSummary `json:"sessions"`
	Total    int                          `json:"total"`
	Limit    int                          `json:"limit"`
	Offset   int                          `json:"offset"`
}

// MarshalMessages serialises the ConversationTurn slice for JSONB storage.
// Returns "[]" for nil/empty slices so the DB column keeps a valid JSON array.
func MarshalMessages(turns []ConversationTurn) ([]byte, error) {
	if len(turns) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(turns)
}

// UnmarshalMessages parses a JSONB column payload into ConversationTurn slices.
// Empty / null payloads yield a nil slice.
func UnmarshalMessages(raw []byte) ([]ConversationTurn, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var out []ConversationTurn
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

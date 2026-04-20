package model

import (
	"encoding/json"
	"time"
)

// ConversationChannel enumerates the transport a conversation used.
type ConversationChannel string

// Supported conversation channels.
const (
	ConversationChannelChat   ConversationChannel = "chat"
	ConversationChannelVoice  ConversationChannel = "voice"
	ConversationChannelWebRTC ConversationChannel = "webrtc"
)

// ConversationMessage is a single message inside a conversation session's
// messages JSONB blob. Role values mirror the OpenAI/Anthropic shape so the
// summarisation worker can forward them verbatim.
type ConversationMessage struct {
	Role      string    `json:"role"` // "user" | "assistant" | "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// ConversationSession captures an end-to-end conversation scoped to an org and KB.
// Populated by chat and voice handlers when a session ends, then consumed by
// the M9 summary job (#257) and the read API (#258).
type ConversationSession struct {
	ID        string                `json:"id"`
	OrgID     string                `json:"org_id"`
	KBID      string                `json:"kb_id"`
	UserID    string                `json:"user_id"` // external identity (Keycloak/SuperTokens sub)
	Channel   ConversationChannel   `json:"channel"`
	Messages  []ConversationMessage `json:"messages"`
	StartedAt time.Time             `json:"started_at"`
	EndedAt   *time.Time            `json:"ended_at,omitempty"`
	Summary   *string               `json:"summary,omitempty"`
}

// MessagesJSON marshals Messages into the JSONB shape stored in Postgres.
func (c *ConversationSession) MessagesJSON() ([]byte, error) {
	if len(c.Messages) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(c.Messages)
}

// EmailSummaryPayload is the Asynq task payload for a conversation summary email.
// It contains only the session id and target recipient — the handler resolves
// everything else via RLS-scoped repository reads.
type EmailSummaryPayload struct {
	OrgID     string `json:"org_id"`
	SessionID string `json:"session_id"`
	UserEmail string `json:"user_email"`
	UserID      string `json:"user_id"`      // for unsubscribe token binding
	WorkspaceID string `json:"workspace_id"` // for preference scoping (admin override)
	UserName    string `json:"user_name,omitempty"`
	KBName      string `json:"kb_name,omitempty"`
}

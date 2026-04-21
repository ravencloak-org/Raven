package model

// EmailSummaryPayload is the Asynq task payload for a conversation summary
// email. It holds only the identifiers the handler needs; everything else
// (messages, channel, kb name) is fetched via RLS-scoped repository reads.
type EmailSummaryPayload struct {
	OrgID       string `json:"org_id"`
	SessionID   string `json:"session_id"`
	UserEmail   string `json:"user_email"`
	UserID      string `json:"user_id"`      // for unsubscribe token binding
	WorkspaceID string `json:"workspace_id"` // for preference scoping (admin override)
	UserName    string `json:"user_name,omitempty"`
	KBName      string `json:"kb_name,omitempty"`
}

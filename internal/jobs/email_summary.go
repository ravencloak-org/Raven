package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/ravencloak-org/Raven/internal/email"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/posthog"
)

// TypeEmailSummary is the Asynq task type for a single conversation-summary email.
const TypeEmailSummary = "notification:email_summary"

// NewEmailSummaryTask constructs a new Asynq task for a conversation-summary email.
func NewEmailSummaryTask(p model.EmailSummaryPayload) (*asynq.Task, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal EmailSummaryPayload: %w", err)
	}
	return asynq.NewTask(TypeEmailSummary, data), nil
}

// SummaryGenerator is the contract the handler needs from the AI worker.
// A call here makes an HTTP POST to /internal/summarize on the Python worker
// and returns a ready-to-send summary.
type SummaryGenerator interface {
	Summarize(ctx context.Context, req SummarizeRequest) (*SummarizeResponse, error)
}

// SummarizeRequest is the payload sent to the AI worker.
type SummarizeRequest struct {
	SessionID string                       `json:"session_id"`
	Channel   string                       `json:"channel"`
	KBName    string                       `json:"kb_name"`
	UserName  string                       `json:"user_name,omitempty"`
	Messages  []model.ConversationMessage  `json:"messages"`
}

// SummarizeResponse is the AI worker's response shape.
type SummarizeResponse struct {
	Subject string   `json:"subject"`
	Bullets []string `json:"bullets"`
	// BodyHTML/Text are optional — when empty the Go handler renders them
	// locally from Bullets using the standard template. That keeps the
	// Python worker responsible only for the AI-specific part.
	BodyHTML string `json:"body_html,omitempty"`
	BodyText string `json:"body_text,omitempty"`
}

// SessionReader is the minimal repository contract the handler needs.
type SessionReader interface {
	GetByID(ctx context.Context, orgID, sessionID string) (*model.ConversationSession, error)
	SetSummary(ctx context.Context, orgID, sessionID, summary string) error
}

// PreferenceChecker is evaluated before sending. Implementations should
// short-circuit to "disabled" for workspaces where the admin has disabled
// summaries, or users who have opted out.
type PreferenceChecker interface {
	IsEnabled(ctx context.Context, orgID, userID, workspaceID string) (bool, error)
}

// EmailSummaryHandlerDeps wires together every collaborator the handler needs.
// Grouping them in a struct lets cmd/worker inject the concrete
// implementations while keeping the handler's signature stable.
type EmailSummaryHandlerDeps struct {
	Logger       *slog.Logger
	Sessions     SessionReader
	Prefs        PreferenceChecker
	Summarizer   SummaryGenerator
	Sender       email.Sender
	PostHog      *posthog.Client
	FrontendBase string // e.g. https://app.ravencloak.io — trailing slash stripped
	SupportEmail string // shown in the email footer
	UnsubSecret  string // HMAC key; env UNSUBSCRIBE_TOKEN_SECRET
	UnsubBaseURL string // e.g. https://api.ravencloak.io/api/v1/notifications/unsubscribe
}

// HandleEmailSummary returns an Asynq handler that renders and sends a single
// conversation summary email. The handler is idempotent per session_id as long
// as callers pass asynq.TaskID for de-duplication (recommended at enqueue time).
func HandleEmailSummary(deps EmailSummaryHandlerDeps) asynq.HandlerFunc {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context, t *asynq.Task) error {
		var p model.EmailSummaryPayload
		if err := json.Unmarshal(t.Payload(), &p); err != nil {
			return fmt.Errorf("email_summary: unmarshal: %v: %w", err, asynq.SkipRetry)
		}
		if p.OrgID == "" || p.SessionID == "" || p.UserEmail == "" || p.UserID == "" {
			return fmt.Errorf("email_summary: incomplete payload: %w", asynq.SkipRetry)
		}

		sess, err := deps.Sessions.GetByID(ctx, p.OrgID, p.SessionID)
		if err != nil {
			return fmt.Errorf("email_summary: load session: %w", err)
		}

		// Resolve preference (workspace_id lives in the session's KB — for now
		// we thread UserID + OrgID; workspace scoping is added by the read API).
		// A nil checker means "always on" — safe default for tests.
		if deps.Prefs != nil {
			workspaceID := p.WorkspaceID
			if workspaceID == "" {
				workspaceID = sess.KBID // best-effort fallback; KB and workspace align 1:1 for single-KB workspaces
			}
			enabled, err := deps.Prefs.IsEnabled(ctx, p.OrgID, p.UserID, workspaceID)
			if err != nil {
				return fmt.Errorf("email_summary: pref check: %w", err)
			}
			if !enabled {
				logger.InfoContext(ctx, "email_summary.skipped",
					slog.String("reason", "preference_disabled"),
					slog.String("session_id", p.SessionID),
				)
				return nil
			}
		}

		// Generate the summary via the AI worker.
		resp, err := deps.Summarizer.Summarize(ctx, SummarizeRequest{
			SessionID: p.SessionID,
			Channel:   string(sess.Channel),
			KBName:    p.KBName,
			UserName:  p.UserName,
			Messages:  sess.Messages,
		})
		if err != nil {
			return fmt.Errorf("email_summary: summarise: %w", err)
		}

		// Persist the summary text for audit + #258's read API.
		if err := deps.Sessions.SetSummary(ctx, p.OrgID, p.SessionID, strings.Join(resp.Bullets, "\n")); err != nil {
			// Non-fatal — continue to send the email.
			logger.WarnContext(ctx, "email_summary.persist_summary_failed",
				slog.String("error", err.Error()),
			)
		}

		unsubSecret := deps.UnsubSecret
		if unsubSecret == "" {
			unsubSecret = os.Getenv(email.UnsubscribeSecretEnv)
		}
		unsubWorkspaceID := p.WorkspaceID
		if unsubWorkspaceID == "" {
			unsubWorkspaceID = sess.KBID
		}
		token, err := email.SignUnsubscribeToken(unsubSecret, p.UserID, unsubWorkspaceID)
		if err != nil {
			return fmt.Errorf("email_summary: sign unsubscribe: %w", err)
		}
		unsubURL := joinURL(deps.UnsubBaseURL, "?token="+token)

		continueURL := buildContinueURL(deps.FrontendBase, sess.KBID, p.SessionID)

		data := email.SummaryTemplateData{
			Greeting:       greetingFromName(p.UserName),
			KBName:         p.KBName,
			ChannelLabel:   channelLabel(sess.Channel),
			SessionDate:    sess.StartedAt,
			Bullets:        resp.Bullets,
			ContinueURL:    continueURL,
			UnsubscribeURL: unsubURL,
			SupportAddress: deps.SupportEmail,
		}
		htmlBody := resp.BodyHTML
		if htmlBody == "" {
			htmlBody = email.RenderSummaryHTML(data)
		}
		textBody := resp.BodyText
		if textBody == "" {
			textBody = email.RenderSummaryText(data)
		}
		subject := resp.Subject
		if subject == "" {
			subject = fmt.Sprintf("Your %s recap — %s", channelLabel(sess.Channel), p.KBName)
		}

		msg := email.Message{
			To:              p.UserEmail,
			Subject:         subject,
			HTMLBody:        htmlBody,
			TextBody:        textBody,
			ListUnsubscribe: unsubURL,
		}
		if err := deps.Sender.Send(ctx, msg); err != nil {
			return fmt.Errorf("email_summary: send: %w", err)
		}

		// Fire-and-forget analytics. PostHog is a no-op when no API key is set.
		if deps.PostHog != nil {
			_ = deps.PostHog.Capture(ctx, p.UserID, "summary_email_sent", map[string]any{
				"channel":       string(sess.Channel),
				"kb_id":         sess.KBID,
				"session_id":    sess.ID,
				"message_count": len(sess.Messages),
				"sent_at":       time.Now().UTC().Format(time.RFC3339),
			})
		}
		return nil
	}
}

// joinURL concatenates a base URL and a trailing query fragment. It tolerates
// an empty base (for tests) and an already-present leading "?".
func joinURL(base, suffix string) string {
	if base == "" {
		return suffix
	}
	u, err := url.Parse(base)
	if err != nil {
		return base + suffix
	}
	q := strings.TrimPrefix(suffix, "?")
	if u.RawQuery != "" {
		u.RawQuery += "&" + q
	} else {
		u.RawQuery = q
	}
	return u.String()
}

// buildContinueURL returns a deep-link into the frontend with UTM params so
// the PostHog `summary_email_clicked` event can be tracked on arrival.
func buildContinueURL(base, kbID, sessionID string) string {
	base = strings.TrimRight(base, "/")
	if base == "" {
		base = "https://app.ravencloak.io"
	}
	v := url.Values{}
	v.Set("session", sessionID)
	v.Set("utm_source", "raven")
	v.Set("utm_medium", "email")
	v.Set("utm_campaign", "summary")
	return fmt.Sprintf("%s/knowledge-bases/%s?%s", base, kbID, v.Encode())
}

// greetingFromName returns "Hi Jobin," or "Hi," when the name is empty.
func greetingFromName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Hi,"
	}
	// Use only the first token to keep greetings friendly, not formal.
	if idx := strings.IndexAny(name, " \t"); idx > 0 {
		name = name[:idx]
	}
	return "Hi " + name + ","
}

func channelLabel(c model.ConversationChannel) string {
	switch c {
	case model.ConversationChannelVoice:
		return "voice call"
	case model.ConversationChannelWebRTC:
		return "voice session"
	default:
		return "chat session"
	}
}

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ConversationRepositoryI is the persistence contract used by
// ConversationService (allows mocking in tests).
type ConversationRepositoryI interface {
	CreateSession(ctx context.Context, orgID, kbID, userID, channel string, initial []model.ConversationTurn) (*model.ConversationSession, error)
	AppendMessage(ctx context.Context, orgID, sessionID string, turn model.ConversationTurn) (*model.ConversationSession, error)
	EndSession(ctx context.Context, orgID, sessionID string) error
	GetRecentByUser(ctx context.Context, orgID, kbID, userID string, limit int) ([]model.ConversationSession, error)
	GetByID(ctx context.Context, orgID, sessionID, userID string) (*model.ConversationSession, error)
	ListByUser(ctx context.Context, orgID, kbID, userID string, limit, offset int) ([]model.ConversationSessionSummary, int, error)
}

// ConversationAnalytics is the subset of the PostHog client used for
// server-side event tracking. Kept as an interface so tests can assert on
// captured events without an HTTP round-trip.
type ConversationAnalytics interface {
	Capture(ctx context.Context, distinctID, event string, properties map[string]any) error
}

// ConversationService orchestrates cross-channel conversation memory —
// session lifecycle, recent-turn history for the AI worker, and
// PostHog server-side event emission.
type ConversationService struct {
	repo      ConversationRepositoryI
	analytics ConversationAnalytics
}

// NewConversationService constructs a ConversationService. analytics may be nil.
func NewConversationService(repo ConversationRepositoryI, analytics ConversationAnalytics) *ConversationService {
	return &ConversationService{repo: repo, analytics: analytics}
}

// StartSession creates a new cross-channel session and emits session_started.
// userID MUST be the authenticated user's stable identity (JWT sub claim).
func (s *ConversationService) StartSession(ctx context.Context, orgID, kbID, userID, channel string) (*model.ConversationSession, error) {
	sess, err := s.repo.CreateSession(ctx, orgID, kbID, userID, channel, nil)
	if err != nil {
		return nil, apierror.NewInternal("failed to start conversation: " + err.Error())
	}

	s.capture(ctx, userID, "session_started", map[string]any{
		"channel":  channel,
		"kb_id":    kbID,
		"org_id":   orgID,
		"$groups":  map[string]any{"organization": orgID},
		"$set":     map[string]any{"org_id": orgID},
	})
	return sess, nil
}

// RecordMessage appends a turn and emits a message_sent event. historyTurnsInjected
// tracks how many prior turns were surfaced to the AI worker for this response.
func (s *ConversationService) RecordMessage(
	ctx context.Context,
	orgID, sessionID, userID, kbID, channel, role, content string,
	isReturningUser bool,
	historyTurnsInjected int,
) (*model.ConversationSession, error) {
	turn := model.ConversationTurn{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().UTC(),
	}
	sess, err := s.repo.AppendMessage(ctx, orgID, sessionID, turn)
	if err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			return nil, apierror.NewNotFound("conversation session not found")
		}
		return nil, apierror.NewInternal("failed to append message: " + err.Error())
	}

	// Track only user-originated turns to avoid doubling the funnel.
	if role == "user" {
		s.capture(ctx, userID, "message_sent", map[string]any{
			"channel":                channel,
			"kb_id":                  kbID,
			"session_id":             sessionID,
			"is_returning_user":      isReturningUser,
			"history_turns_injected": historyTurnsInjected,
			"$groups":                map[string]any{"organization": orgID},
		})
	}
	return sess, nil
}

// EndSession finalises a session and emits session_ended with summary metrics.
func (s *ConversationService) EndSession(ctx context.Context, orgID, sessionID, userID, kbID, channel string, messageCount int, startedAt time.Time) error {
	if err := s.repo.EndSession(ctx, orgID, sessionID); err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			return apierror.NewNotFound("conversation session not found")
		}
		return apierror.NewInternal("failed to end conversation: " + err.Error())
	}

	duration := 0
	if !startedAt.IsZero() {
		duration = int(time.Since(startedAt).Seconds())
		if duration < 0 {
			duration = 0
		}
	}

	s.capture(ctx, userID, "session_ended", map[string]any{
		"channel":          channel,
		"kb_id":            kbID,
		"session_id":       sessionID,
		"message_count":    messageCount,
		"duration_seconds": duration,
		"$groups":          map[string]any{"organization": orgID},
	})
	return nil
}

// RecentHistory returns the last N turns across the user's most recent
// sessions on the KB, flattened in chronological order. This is the payload
// injected into AI worker requests as `conversation_history`.
//
// maxTurns defaults to MaxConversationHistoryTurns when <= 0.
func (s *ConversationService) RecentHistory(ctx context.Context, orgID, kbID, userID string, maxTurns int) ([]model.ConversationTurn, error) {
	if userID == "" {
		return nil, nil
	}
	if maxTurns <= 0 {
		maxTurns = model.MaxConversationHistoryTurns
	}

	// Pull a small window of recent sessions; we flatten their messages and
	// keep only the tail so the total number of turns stays bounded.
	sessions, err := s.repo.GetRecentByUser(ctx, orgID, kbID, userID, 3)
	if err != nil {
		return nil, fmt.Errorf("RecentHistory: %w", err)
	}

	// Sessions come back newest-first, so walk them oldest-first when
	// assembling chronological turns.
	var all []model.ConversationTurn
	for i := len(sessions) - 1; i >= 0; i-- {
		all = append(all, sessions[i].Messages...)
	}

	if len(all) > maxTurns {
		all = all[len(all)-maxTurns:]
	}
	return all, nil
}

// IsReturningUser reports whether the user has any prior session on this KB.
func (s *ConversationService) IsReturningUser(ctx context.Context, orgID, kbID, userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	sessions, err := s.repo.GetRecentByUser(ctx, orgID, kbID, userID, 1)
	if err != nil {
		return false, err
	}
	return len(sessions) > 0, nil
}

// ListForUser returns paginated session summaries for the authenticated user
// on a KB.
func (s *ConversationService) ListForUser(ctx context.Context, orgID, kbID, userID string, limit, offset int) (*model.ConversationListResponse, error) {
	summaries, total, err := s.repo.ListByUser(ctx, orgID, kbID, userID, limit, offset)
	if err != nil {
		return nil, apierror.NewInternal("failed to list conversations: " + err.Error())
	}
	return &model.ConversationListResponse{
		Sessions: summaries,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}, nil
}

// GetTranscript returns a full conversation transcript scoped to the authenticated user.
func (s *ConversationService) GetTranscript(ctx context.Context, orgID, kbID, sessionID, userID string) (*model.ConversationSession, error) {
	sess, err := s.repo.GetByID(ctx, orgID, sessionID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			return nil, apierror.NewNotFound("conversation session not found")
		}
		return nil, apierror.NewInternal("failed to fetch transcript: " + err.Error())
	}
	// Enforce KB ownership in addition to user_id / org_id.
	if kbID != "" && sess.KBID != kbID {
		return nil, apierror.NewNotFound("conversation session not found")
	}
	return sess, nil
}

// capture fires a PostHog event without blocking the request path. Errors are
// logged and swallowed — analytics must never surface as user-facing failures.
func (s *ConversationService) capture(ctx context.Context, distinctID, event string, props map[string]any) {
	if s.analytics == nil || distinctID == "" {
		return
	}
	go func(ctx context.Context) {
		// Detach from the request context so cancellation doesn't drop the event.
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.analytics.Capture(bg, distinctID, event, props); err != nil {
			slog.WarnContext(ctx, "posthog capture failed",
				slog.String("event", event),
				slog.String("error", err.Error()),
			)
		}
	}(ctx)
}

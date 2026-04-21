package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// ConversationRepositoryI is the persistence contract used by
// ConversationService (allows mocking in tests).
//
// EndSession returns (ended=true, nil) only when the row actually transitioned
// from open to ended; idempotent repeat calls return (false, nil).
type ConversationRepositoryI interface {
	CreateSession(ctx context.Context, orgID, kbID, userID, channel string, initial []model.ConversationTurn) (*model.ConversationSession, error)
	AppendMessage(ctx context.Context, orgID, sessionID string, turn model.ConversationTurn) (*model.ConversationSession, error)
	EndSession(ctx context.Context, orgID, sessionID string) (bool, error)
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

// analyticsQueueSize is the buffer depth for the PostHog capture pipeline.
// A full queue drops events (with a rate-limited warning) rather than
// blocking the request path or spawning unbounded goroutines.
const analyticsQueueSize = 256

// analyticsWorkerCount is the number of background workers draining the
// capture queue. Small because capture is latency-insensitive.
const analyticsWorkerCount = 4

// recentSessionsScanWindow is how many of the user's most recent sessions
// RecentHistory scans to assemble up to maxTurns worth of cross-channel
// history. Heuristic: newer sessions are sharpest; scanning more rarely
// adds signal and costs DB round-trips.
const recentSessionsScanWindow = 3

// captureJob is a queued PostHog event carrying a defensive copy of its
// props map so a single map is never shared across goroutine boundaries.
type captureJob struct {
	distinctID string
	event      string
	props      map[string]any
}

// ConversationService orchestrates cross-channel conversation memory —
// session lifecycle, recent-turn history for the AI worker, and
// PostHog server-side event emission.
type ConversationService struct {
	repo      ConversationRepositoryI
	analytics ConversationAnalytics
	// capQueue is a bounded channel drained by a fixed-size worker pool.
	// Nil when analytics is nil.
	capQueue chan captureJob
	// dropLogOnce rate-limits "queue full" warnings so a flood of drops does
	// not saturate the log pipeline.
	dropLogOnce atomic.Int64
}

// NewConversationService constructs a ConversationService. analytics may be nil.
// When non-nil, a small background worker pool is started to drain PostHog
// events without spawning unbounded goroutines on the request path.
func NewConversationService(repo ConversationRepositoryI, analytics ConversationAnalytics) *ConversationService {
	s := &ConversationService{repo: repo, analytics: analytics}
	if analytics != nil {
		s.capQueue = make(chan captureJob, analyticsQueueSize)
		for i := 0; i < analyticsWorkerCount; i++ {
			go s.captureWorker()
		}
	}
	return s
}

// captureWorker drains the PostHog event queue. It runs for the lifetime of
// the process; there is no shutdown hook today because the API process exits
// by SIGTERM and losing an in-flight analytics event is acceptable.
func (s *ConversationService) captureWorker() {
	for job := range s.capQueue {
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := s.analytics.Capture(bg, job.distinctID, job.event, job.props); err != nil {
			slog.Warn("posthog capture failed",
				slog.String("event", job.event),
				slog.String("error", err.Error()),
			)
		}
		cancel()
	}
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
// The session_ended PostHog event is only emitted when the underlying row
// actually transitioned from open to ended; subsequent idempotent calls are
// a silent no-op so analytics does not double-count.
func (s *ConversationService) EndSession(ctx context.Context, orgID, sessionID, userID, kbID, channel string, messageCount int, startedAt time.Time) error {
	ended, err := s.repo.EndSession(ctx, orgID, sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			return apierror.NewNotFound("conversation session not found")
		}
		return apierror.NewInternal("failed to end conversation: " + err.Error())
	}
	if !ended {
		// Already ended on a prior call; skip the funnel event.
		return nil
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
	sessions, err := s.repo.GetRecentByUser(ctx, orgID, kbID, userID, recentSessionsScanWindow)
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
// on a KB. The response echoes the clamped limit/offset actually used by the
// repository so clients aren't misled by "limit: 1000" when only 20 rows came
// back.
func (s *ConversationService) ListForUser(ctx context.Context, orgID, kbID, userID string, limit, offset int) (*model.ConversationListResponse, error) {
	appliedLimit, appliedOffset := clampConversationPagination(limit, offset)
	summaries, total, err := s.repo.ListByUser(ctx, orgID, kbID, userID, appliedLimit, appliedOffset)
	if err != nil {
		return nil, apierror.NewInternal("failed to list conversations: " + err.Error())
	}
	return &model.ConversationListResponse{
		Sessions: summaries,
		Total:    total,
		Limit:    appliedLimit,
		Offset:   appliedOffset,
	}, nil
}

// clampConversationPagination mirrors the repo-side clamp so the service can
// report back the values that were actually applied. Kept in sync with
// repository.ListByUser.
func clampConversationPagination(limit, offset int) (int, int) {
	if limit <= 0 || limit > 100 {
		limit = model.DefaultConversationListLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
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

// capture fires a PostHog event without blocking the request path. Events
// are handed to a bounded worker pool; when the queue is full the event is
// dropped (rate-limited warning) rather than blocking or spawning a fresh
// goroutine. The props map is cloned before enqueue so the queued job owns
// its data and no map is ever shared across goroutine boundaries.
func (s *ConversationService) capture(ctx context.Context, distinctID, event string, props map[string]any) {
	if s.analytics == nil || distinctID == "" || s.capQueue == nil {
		return
	}
	job := captureJob{
		distinctID: distinctID,
		event:      event,
		props:      cloneStringAnyMap(props),
	}
	select {
	case s.capQueue <- job:
	default:
		// Queue full — drop. Rate-limit the warning: only log every 1024 drops.
		if n := s.dropLogOnce.Add(1); n%1024 == 1 {
			slog.WarnContext(ctx, "posthog capture queue full, event dropped",
				slog.String("event", event),
				slog.Int64("total_drops", n),
			)
		}
	}
}

// cloneStringAnyMap returns a shallow copy of m. Nested maps/slices are not
// deep-copied — callers MUST NOT mutate the original map values after handing
// the clone to a goroutine. All current call-sites build a fresh literal.
func cloneStringAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// VoiceRepository defines the persistence interface the VoiceService requires.
type VoiceRepository interface {
	CreateSession(ctx context.Context, tx pgx.Tx, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error)
	GetSession(ctx context.Context, tx pgx.Tx, orgID, sessionID string) (*model.VoiceSession, error)
	UpdateSessionState(ctx context.Context, tx pgx.Tx, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error)
	ListSessions(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.VoiceSession, int, error)
	AppendTurn(ctx context.Context, tx pgx.Tx, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error)
	ListTurns(ctx context.Context, tx pgx.Tx, orgID, sessionID string) ([]model.VoiceTurn, error)
	CountActiveSessions(ctx context.Context, tx pgx.Tx, orgID string) (int, error)
}

// LiveKitClient defines the interface for LiveKit operations so it can be
// mocked in tests.
type LiveKitClient interface {
	CreateRoom(ctx context.Context, name, metadata string) error
	DeleteRoom(ctx context.Context, name string) error
	GenerateToken(roomName, participantIdentity, participantName string) (string, error)
}

// VoiceService contains business logic for voice session lifecycle and transcription storage.
type VoiceService struct {
	repo   VoiceRepository
	pool   *pgxpool.Pool
	lkc    LiveKitClient
	lkHost string // LiveKit host URL for token responses
	// maxConcurrentSessions is the default limit (Free plan = 1). Set to -1 for unlimited.
	maxConcurrentSessions int
}

// NewVoiceService creates a new VoiceService. The livekit client may be nil if
// LiveKit is not configured (room operations become no-ops).
// maxSessions controls the concurrent session cap per org (1 = free, -1 = unlimited).
func NewVoiceService(repo VoiceRepository, pool *pgxpool.Pool, lkc LiveKitClient, lkHost string, maxSessions int) *VoiceService {
	if maxSessions == 0 {
		maxSessions = 1 // default to free tier
	}
	return &VoiceService{
		repo:                  repo,
		pool:                  pool,
		lkc:                   lkc,
		lkHost:                lkHost,
		maxConcurrentSessions: maxSessions,
	}
}

// generateRoomName produces a deterministic, human-friendly room name like
// "voice-ab12-cd34" using short prefixes of the orgID and a random UUID.
func generateRoomName(orgID string) string {
	orgShort := orgID
	if len(orgShort) > 4 {
		orgShort = orgShort[:4]
	}
	uuidShort := uuid.New().String()[:8]
	return fmt.Sprintf("voice-%s-%s", orgShort, uuidShort)
}

// CreateSession creates a new voice session in the 'created' state.
// It auto-generates a LiveKit room name, enforces concurrent session limits,
// and creates the room on the LiveKit server.
func (s *VoiceService) CreateSession(ctx context.Context, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error) {
	if req == nil {
		return nil, apierror.NewBadRequest("request body must not be nil")
	}

	// Auto-generate room name.
	req.LiveKitRoom = generateRoomName(orgID)

	var session *model.VoiceSession
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Check concurrent session limit with advisory lock to prevent races.
		if s.maxConcurrentSessions >= 0 {
			// Acquire a transaction-scoped advisory lock keyed on a hash of the orgID
			// to serialize concurrent session-create requests for the same org.
			lockKey := int64(fnv32a(orgID))
			if _, e := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); e != nil {
				return e
			}
			active, e := s.repo.CountActiveSessions(ctx, tx, orgID)
			if e != nil {
				return e
			}
			if active >= s.maxConcurrentSessions {
				return apierror.NewTooManyRequests("concurrent voice session limit reached")
			}
		}

		var e error
		session, e = s.repo.CreateSession(ctx, tx, orgID, req)
		return e
	})
	if err != nil {
		if isTooManyRequests(err) {
			return nil, err
		}
		slog.ErrorContext(ctx, "VoiceService.CreateSession db error", "error", err)
		return nil, apierror.NewInternal("failed to create voice session")
	}

	// Create the room on LiveKit (best-effort; session exists in DB even if this fails).
	if s.lkc != nil {
		if err := s.lkc.CreateRoom(ctx, session.LiveKitRoom, ""); err != nil {
			slog.ErrorContext(ctx, "VoiceService.CreateSession: failed to create LiveKit room",
				"room", session.LiveKitRoom, "error", err)
		}
	}

	return session, nil
}

// GetSession retrieves a voice session by ID.
func (s *VoiceService) GetSession(ctx context.Context, orgID, sessionID string) (*model.VoiceSession, error) {
	var session *model.VoiceSession
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		session, e = s.repo.GetSession(ctx, tx, orgID, sessionID)
		return e
	})
	if err != nil {
		if isVoiceNotFound(err) {
			return nil, apierror.NewNotFound("voice session not found")
		}
		slog.ErrorContext(ctx, "VoiceService.GetSession db error", "error", err)
		return nil, apierror.NewInternal("failed to get voice session")
	}
	return session, nil
}

// UpdateSessionState transitions a session to active or ended.
// Transitioning to 'active' sets started_at; transitioning to 'ended' sets ended_at
// and deletes the LiveKit room.
func (s *VoiceService) UpdateSessionState(ctx context.Context, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error) {
	if state != model.VoiceSessionStateActive && state != model.VoiceSessionStateEnded {
		return nil, apierror.NewBadRequest("state must be 'active' or 'ended'")
	}

	var session *model.VoiceSession
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		current, e := s.repo.GetSession(ctx, tx, orgID, sessionID)
		if e != nil {
			return e
		}
		if current.State == model.VoiceSessionStateEnded {
			return apierror.NewBadRequest("cannot transition from ended state: session is terminal")
		}
		session, e = s.repo.UpdateSessionState(ctx, tx, orgID, sessionID, state)
		return e
	})
	if err != nil {
		if isVoiceNotFound(err) {
			return nil, apierror.NewNotFound("voice session not found")
		}
		slog.ErrorContext(ctx, "VoiceService.UpdateSessionState db error", "error", err)
		return nil, apierror.NewInternal("failed to update voice session state")
	}

	// When a session ends, clean up the LiveKit room. Log but don't fail.
	if state == model.VoiceSessionStateEnded && s.lkc != nil {
		if err := s.lkc.DeleteRoom(ctx, session.LiveKitRoom); err != nil {
			slog.ErrorContext(ctx, "VoiceService.UpdateSessionState: failed to delete LiveKit room",
				"room", session.LiveKitRoom, "error", err)
		}
	}

	return session, nil
}

// ListSessions returns org-scoped voice sessions with pagination.
func (s *VoiceService) ListSessions(ctx context.Context, orgID string, limit, offset int) (*model.VoiceSessionListResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var sessions []model.VoiceSession
	var total int
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		sessions, total, e = s.repo.ListSessions(ctx, tx, orgID, limit, offset)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "VoiceService.ListSessions db error", "error", err)
		return nil, apierror.NewInternal("failed to list voice sessions")
	}
	if sessions == nil {
		sessions = []model.VoiceSession{}
	}
	return &model.VoiceSessionListResponse{
		Sessions: sessions,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}, nil
}

// GenerateToken generates a LiveKit access token for a voice session.
// The session must exist and be in 'created' or 'active' state.
func (s *VoiceService) GenerateToken(ctx context.Context, orgID, sessionID, identity string) (*model.VoiceTokenResponse, error) {
	if s.lkc == nil {
		return nil, apierror.NewInternal("LiveKit is not configured")
	}

	var session *model.VoiceSession
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		session, e = s.repo.GetSession(ctx, tx, orgID, sessionID)
		return e
	})
	if err != nil {
		if isVoiceNotFound(err) {
			return nil, apierror.NewNotFound("voice session not found")
		}
		slog.ErrorContext(ctx, "VoiceService.GenerateToken db error", "error", err)
		return nil, apierror.NewInternal("failed to get voice session")
	}

	if session.State == model.VoiceSessionStateEnded {
		return nil, apierror.NewBadRequest("cannot generate token for ended session")
	}

	token, err := s.lkc.GenerateToken(session.LiveKitRoom, identity, identity)
	if err != nil {
		slog.ErrorContext(ctx, "VoiceService.GenerateToken: token generation failed", "error", err)
		return nil, apierror.NewInternal("failed to generate LiveKit token")
	}

	return &model.VoiceTokenResponse{
		Token: token,
		URL:   s.lkHost,
	}, nil
}

// AppendTurn stores a transcribed turn for an existing voice session.
func (s *VoiceService) AppendTurn(ctx context.Context, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error) {
	if req == nil {
		return nil, apierror.NewBadRequest("request body must not be nil")
	}
	var turn *model.VoiceTurn
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Verify the session exists and belongs to this org before appending.
		if _, e := s.repo.GetSession(ctx, tx, orgID, sessionID); e != nil {
			return e
		}
		var e error
		turn, e = s.repo.AppendTurn(ctx, tx, orgID, sessionID, req)
		return e
	})
	if err != nil {
		if isVoiceNotFound(err) {
			return nil, apierror.NewNotFound("voice session not found")
		}
		slog.ErrorContext(ctx, "VoiceService.AppendTurn db error", "error", err)
		return nil, apierror.NewInternal("failed to append voice turn")
	}
	return turn, nil
}

// ListTurns returns all transcription turns for a session ordered by started_at.
func (s *VoiceService) ListTurns(ctx context.Context, orgID, sessionID string) (*model.VoiceTurnListResponse, error) {
	var turns []model.VoiceTurn
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		// Verify session ownership before listing turns.
		if _, e := s.repo.GetSession(ctx, tx, orgID, sessionID); e != nil {
			return e
		}
		var e error
		turns, e = s.repo.ListTurns(ctx, tx, orgID, sessionID)
		return e
	})
	if err != nil {
		if isVoiceNotFound(err) {
			return nil, apierror.NewNotFound("voice session not found")
		}
		slog.ErrorContext(ctx, "VoiceService.ListTurns db error", "error", err)
		return nil, apierror.NewInternal("failed to list voice turns")
	}
	if turns == nil {
		turns = []model.VoiceTurn{}
	}
	return &model.VoiceTurnListResponse{
		SessionID: sessionID,
		Turns:     turns,
	}, nil
}

// isVoiceNotFound checks if an error indicates a missing record.
func isVoiceNotFound(err error) bool {
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

// isTooManyRequests checks if the error is a 429 Too Many Requests error.
func isTooManyRequests(err error) bool {
	if err == nil {
		return false
	}
	var appErr *apierror.AppError
	if errors.As(err, &appErr) {
		return appErr.Code == 429
	}
	return false
}

// fnv32a produces a 32-bit FNV-1a hash for advisory lock keys.
func fnv32a(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}

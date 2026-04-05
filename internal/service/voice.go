package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
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
}

// VoiceService contains business logic for voice session lifecycle and transcription storage.
type VoiceService struct {
	repo *repository.VoiceRepository
	pool *pgxpool.Pool
}

// NewVoiceService creates a new VoiceService.
func NewVoiceService(repo *repository.VoiceRepository, pool *pgxpool.Pool) *VoiceService {
	return &VoiceService{repo: repo, pool: pool}
}

// CreateSession creates a new voice session in the 'created' state.
func (s *VoiceService) CreateSession(ctx context.Context, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error) {
	var session *model.VoiceSession
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		session, e = s.repo.CreateSession(ctx, tx, orgID, req)
		return e
	})
	if err != nil {
		slog.ErrorContext(ctx, "VoiceService.CreateSession db error", "error", err)
		return nil, apierror.NewInternal("failed to create voice session")
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
// Transitioning to 'active' sets started_at; transitioning to 'ended' sets ended_at.
func (s *VoiceService) UpdateSessionState(ctx context.Context, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error) {
	if state != model.VoiceSessionStateActive && state != model.VoiceSessionStateEnded {
		return nil, apierror.NewBadRequest("state must be 'active' or 'ended'")
	}

	var session *model.VoiceSession
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
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

// AppendTurn stores a transcribed turn for an existing voice session.
func (s *VoiceService) AppendTurn(ctx context.Context, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error) {
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
	msg := err.Error()
	return strings.Contains(msg, "no rows in result set") ||
		strings.HasSuffix(msg, "not found")
}

package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/model"
)

// VoiceRepository handles database operations for voice sessions and turns.
// All mutating operations run inside a pgx.Tx with org_id set for RLS.
type VoiceRepository struct {
	pool *pgxpool.Pool
}

// NewVoiceRepository creates a new VoiceRepository.
func NewVoiceRepository(pool *pgxpool.Pool) *VoiceRepository {
	return &VoiceRepository{pool: pool}
}

const (
	sqlVoiceSessionInsert = `
		INSERT INTO voice_sessions (org_id, user_id, stranger_id, livekit_room)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, user_id, stranger_id, livekit_room, state,
		          started_at, ended_at, call_duration_seconds, created_at, updated_at`

	sqlVoiceSessionByID = `
		SELECT id, org_id, user_id, stranger_id, livekit_room, state,
		       started_at, ended_at, call_duration_seconds, created_at, updated_at
		FROM voice_sessions
		WHERE id = $1 AND org_id = $2`

	sqlVoiceSessionUpdateState = `
		UPDATE voice_sessions SET
			state      = $3,
			started_at = CASE WHEN $3::voice_session_state = 'active'  THEN NOW()    ELSE started_at END,
			ended_at   = CASE WHEN $3::voice_session_state = 'ended'   THEN NOW()    ELSE ended_at   END
		WHERE id = $1 AND org_id = $2
		RETURNING id, org_id, user_id, stranger_id, livekit_room, state,
		          started_at, ended_at, call_duration_seconds, created_at, updated_at`

	sqlVoiceSessionList = `
		SELECT id, org_id, user_id, stranger_id, livekit_room, state,
		       started_at, ended_at, call_duration_seconds, created_at, updated_at
		FROM voice_sessions
		WHERE org_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	sqlVoiceSessionCount = `SELECT COUNT(*) FROM voice_sessions WHERE org_id = $1`

	sqlVoiceTurnInsert = `
		INSERT INTO voice_turns (session_id, org_id, speaker, transcript, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, session_id, org_id, speaker, transcript, started_at, ended_at, created_at`

	sqlVoiceTurnList = `
		SELECT id, session_id, org_id, speaker, transcript, started_at, ended_at, created_at
		FROM voice_turns
		WHERE session_id = $1 AND org_id = $2
		ORDER BY started_at ASC`
)

func scanVoiceSession(row pgx.Row) (*model.VoiceSession, error) {
	var s model.VoiceSession
	err := row.Scan(
		&s.ID,
		&s.OrgID,
		&s.UserID,
		&s.StrangerID,
		&s.LiveKitRoom,
		&s.State,
		&s.StartedAt,
		&s.EndedAt,
		&s.CallDurationSeconds,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func scanVoiceTurn(row pgx.Row) (*model.VoiceTurn, error) {
	var t model.VoiceTurn
	err := row.Scan(
		&t.ID,
		&t.SessionID,
		&t.OrgID,
		&t.Speaker,
		&t.Transcript,
		&t.StartedAt,
		&t.EndedAt,
		&t.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateSession inserts a new voice session and returns it with DB-assigned fields.
func (r *VoiceRepository) CreateSession(ctx context.Context, tx pgx.Tx, orgID string, req *model.CreateVoiceSessionRequest) (*model.VoiceSession, error) {
	row := tx.QueryRow(ctx, sqlVoiceSessionInsert,
		orgID,
		req.UserID,
		req.StrangerID,
		req.LiveKitRoom,
	)
	s, err := scanVoiceSession(row)
	if err != nil {
		return nil, fmt.Errorf("VoiceRepository.CreateSession: %w", err)
	}
	return s, nil
}

// GetSession retrieves a voice session by ID within an org.
func (r *VoiceRepository) GetSession(ctx context.Context, tx pgx.Tx, orgID, sessionID string) (*model.VoiceSession, error) {
	row := tx.QueryRow(ctx, sqlVoiceSessionByID, sessionID, orgID)
	s, err := scanVoiceSession(row)
	if err != nil {
		return nil, fmt.Errorf("VoiceRepository.GetSession: %w", err)
	}
	return s, nil
}

// UpdateSessionState transitions a session to the given state and sets
// started_at/ended_at automatically based on the new state.
func (r *VoiceRepository) UpdateSessionState(ctx context.Context, tx pgx.Tx, orgID, sessionID string, state model.VoiceSessionState) (*model.VoiceSession, error) {
	row := tx.QueryRow(ctx, sqlVoiceSessionUpdateState, sessionID, orgID, state)
	s, err := scanVoiceSession(row)
	if err != nil {
		return nil, fmt.Errorf("VoiceRepository.UpdateSessionState: %w", err)
	}
	return s, nil
}

// ListSessions returns voice sessions for an org ordered by created_at DESC.
func (r *VoiceRepository) ListSessions(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.VoiceSession, int, error) {
	var total int
	if err := tx.QueryRow(ctx, sqlVoiceSessionCount, orgID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("VoiceRepository.ListSessions count: %w", err)
	}

	rows, err := tx.Query(ctx, sqlVoiceSessionList, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("VoiceRepository.ListSessions query: %w", err)
	}
	defer rows.Close()

	var sessions []model.VoiceSession
	for rows.Next() {
		var s model.VoiceSession
		if err := rows.Scan(
			&s.ID,
			&s.OrgID,
			&s.UserID,
			&s.StrangerID,
			&s.LiveKitRoom,
			&s.State,
			&s.StartedAt,
			&s.EndedAt,
			&s.CallDurationSeconds,
			&s.CreatedAt,
			&s.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("VoiceRepository.ListSessions scan: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, total, rows.Err()
}

// AppendTurn inserts a new transcription turn for a session.
func (r *VoiceRepository) AppendTurn(ctx context.Context, tx pgx.Tx, orgID, sessionID string, req *model.AppendVoiceTurnRequest) (*model.VoiceTurn, error) {
	row := tx.QueryRow(ctx, sqlVoiceTurnInsert,
		sessionID,
		orgID,
		req.Speaker,
		req.Transcript,
		req.StartedAt,
		req.EndedAt,
	)
	t, err := scanVoiceTurn(row)
	if err != nil {
		return nil, fmt.Errorf("VoiceRepository.AppendTurn: %w", err)
	}
	return t, nil
}

// ListTurns returns all turns for a session ordered by started_at ASC.
func (r *VoiceRepository) ListTurns(ctx context.Context, tx pgx.Tx, orgID, sessionID string) ([]model.VoiceTurn, error) {
	rows, err := tx.Query(ctx, sqlVoiceTurnList, sessionID, orgID)
	if err != nil {
		return nil, fmt.Errorf("VoiceRepository.ListTurns: %w", err)
	}
	defer rows.Close()

	var turns []model.VoiceTurn
	for rows.Next() {
		var t model.VoiceTurn
		if err := rows.Scan(
			&t.ID,
			&t.SessionID,
			&t.OrgID,
			&t.Speaker,
			&t.Transcript,
			&t.StartedAt,
			&t.EndedAt,
			&t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("VoiceRepository.ListTurns scan: %w", err)
		}
		turns = append(turns, t)
	}
	return turns, rows.Err()
}

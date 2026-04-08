package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/model"
)

// WhatsAppRepository handles database operations for WhatsApp phone numbers and calls.
// All mutating operations run inside a pgx.Tx with org_id set for RLS.
type WhatsAppRepository struct {
	pool *pgxpool.Pool
}

// NewWhatsAppRepository creates a new WhatsAppRepository.
func NewWhatsAppRepository(pool *pgxpool.Pool) *WhatsAppRepository {
	return &WhatsAppRepository{pool: pool}
}

// --- SQL constants ---

const (
	sqlWAPhoneInsert = `
		INSERT INTO whatsapp_phone_numbers (org_id, phone_number, display_name, waba_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, phone_number, display_name, waba_id, verified, created_at, updated_at`

	sqlWAPhoneByID = `
		SELECT id, org_id, phone_number, display_name, waba_id, verified, created_at, updated_at
		FROM whatsapp_phone_numbers
		WHERE id = $1 AND org_id = $2`

	sqlWAPhoneUpdate = `
		UPDATE whatsapp_phone_numbers SET
			display_name = COALESCE($3, display_name),
			verified     = COALESCE($4, verified)
		WHERE id = $1 AND org_id = $2
		RETURNING id, org_id, phone_number, display_name, waba_id, verified, created_at, updated_at`

	sqlWAPhoneDelete = `DELETE FROM whatsapp_phone_numbers WHERE id = $1 AND org_id = $2`

	sqlWAPhoneList = `
		SELECT id, org_id, phone_number, display_name, waba_id, verified, created_at, updated_at
		FROM whatsapp_phone_numbers
		WHERE org_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	sqlWAPhoneCount = `SELECT COUNT(*) FROM whatsapp_phone_numbers WHERE org_id = $1`

	sqlWACallInsert = `
		INSERT INTO whatsapp_calls (org_id, call_id, phone_number_id, direction, state, caller, callee, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, org_id, call_id, phone_number_id, direction, state,
		          caller, callee, started_at, ended_at, duration_seconds, created_at, updated_at`

	sqlWACallByID = `
		SELECT id, org_id, call_id, phone_number_id, direction, state,
		       caller, callee, started_at, ended_at, duration_seconds, created_at, updated_at
		FROM whatsapp_calls
		WHERE id = $1 AND org_id = $2`

	sqlWACallByCallID = `
		SELECT id, org_id, call_id, phone_number_id, direction, state,
		       caller, callee, started_at, ended_at, duration_seconds, created_at, updated_at
		FROM whatsapp_calls
		WHERE call_id = $1 AND org_id = $2`

	sqlWACallUpdateState = `
		UPDATE whatsapp_calls SET
			state      = $3,
			started_at = CASE WHEN $3::whatsapp_call_state = 'connected' AND started_at IS NULL THEN NOW() ELSE started_at END,
			ended_at   = CASE WHEN $3::whatsapp_call_state = 'ended' THEN NOW() ELSE ended_at END
		WHERE id = $1 AND org_id = $2
		RETURNING id, org_id, call_id, phone_number_id, direction, state,
		          caller, callee, started_at, ended_at, duration_seconds, created_at, updated_at`

	sqlWACallList = `
		SELECT id, org_id, call_id, phone_number_id, direction, state,
		       caller, callee, started_at, ended_at, duration_seconds, created_at, updated_at
		FROM whatsapp_calls
		WHERE org_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	sqlWACallCount = `SELECT COUNT(*) FROM whatsapp_calls WHERE org_id = $1`
)

// --- Scan helpers ---

func scanWAPhone(row pgx.Row) (*model.WhatsAppPhoneNumber, error) {
	var p model.WhatsAppPhoneNumber
	err := row.Scan(
		&p.ID,
		&p.OrgID,
		&p.PhoneNumber,
		&p.DisplayName,
		&p.WABAID,
		&p.Verified,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanWACall(row pgx.Row) (*model.WhatsAppCall, error) {
	var c model.WhatsAppCall
	err := row.Scan(
		&c.ID,
		&c.OrgID,
		&c.CallID,
		&c.PhoneNumberID,
		&c.Direction,
		&c.State,
		&c.Caller,
		&c.Callee,
		&c.StartedAt,
		&c.EndedAt,
		&c.DurationSeconds,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// --- Phone number CRUD ---

// CreatePhoneNumber inserts a new WhatsApp phone number.
func (r *WhatsAppRepository) CreatePhoneNumber(ctx context.Context, tx pgx.Tx, orgID string, req *model.CreateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error) {
	row := tx.QueryRow(ctx, sqlWAPhoneInsert,
		orgID,
		req.PhoneNumber,
		req.DisplayName,
		req.WABAID,
	)
	p, err := scanWAPhone(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppRepository.CreatePhoneNumber: %w", err)
	}
	return p, nil
}

// GetPhoneNumber retrieves a phone number by ID within an org.
func (r *WhatsAppRepository) GetPhoneNumber(ctx context.Context, tx pgx.Tx, orgID, phoneID string) (*model.WhatsAppPhoneNumber, error) {
	row := tx.QueryRow(ctx, sqlWAPhoneByID, phoneID, orgID)
	p, err := scanWAPhone(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppRepository.GetPhoneNumber: %w", err)
	}
	return p, nil
}

// UpdatePhoneNumber updates mutable fields of a phone number.
func (r *WhatsAppRepository) UpdatePhoneNumber(ctx context.Context, tx pgx.Tx, orgID, phoneID string, req *model.UpdateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error) {
	row := tx.QueryRow(ctx, sqlWAPhoneUpdate, phoneID, orgID, req.DisplayName, req.Verified)
	p, err := scanWAPhone(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppRepository.UpdatePhoneNumber: %w", err)
	}
	return p, nil
}

// DeletePhoneNumber removes a phone number by ID.
func (r *WhatsAppRepository) DeletePhoneNumber(ctx context.Context, tx pgx.Tx, orgID, phoneID string) error {
	tag, err := tx.Exec(ctx, sqlWAPhoneDelete, phoneID, orgID)
	if err != nil {
		return fmt.Errorf("WhatsAppRepository.DeletePhoneNumber: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListPhoneNumbers returns phone numbers for an org with pagination.
func (r *WhatsAppRepository) ListPhoneNumbers(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.WhatsAppPhoneNumber, int, error) {
	var total int
	if err := tx.QueryRow(ctx, sqlWAPhoneCount, orgID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("WhatsAppRepository.ListPhoneNumbers count: %w", err)
	}

	rows, err := tx.Query(ctx, sqlWAPhoneList, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("WhatsAppRepository.ListPhoneNumbers query: %w", err)
	}
	defer rows.Close()

	var phones []model.WhatsAppPhoneNumber
	for rows.Next() {
		var p model.WhatsAppPhoneNumber
		if err := rows.Scan(
			&p.ID,
			&p.OrgID,
			&p.PhoneNumber,
			&p.DisplayName,
			&p.WABAID,
			&p.Verified,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("WhatsAppRepository.ListPhoneNumbers scan: %w", err)
		}
		phones = append(phones, p)
	}
	return phones, total, rows.Err()
}

// --- Call CRUD ---

// CreateCall inserts a new WhatsApp call record.
func (r *WhatsAppRepository) CreateCall(ctx context.Context, tx pgx.Tx, orgID string, call *model.WhatsAppCall) (*model.WhatsAppCall, error) {
	row := tx.QueryRow(ctx, sqlWACallInsert,
		orgID,
		call.CallID,
		call.PhoneNumberID,
		call.Direction,
		call.State,
		call.Caller,
		call.Callee,
		call.StartedAt,
	)
	c, err := scanWACall(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppRepository.CreateCall: %w", err)
	}
	return c, nil
}

// GetCall retrieves a call by ID within an org.
func (r *WhatsAppRepository) GetCall(ctx context.Context, tx pgx.Tx, orgID, callID string) (*model.WhatsAppCall, error) {
	row := tx.QueryRow(ctx, sqlWACallByID, callID, orgID)
	c, err := scanWACall(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppRepository.GetCall: %w", err)
	}
	return c, nil
}

// GetCallByCallID retrieves a call by Meta's external call_id within an org.
func (r *WhatsAppRepository) GetCallByCallID(ctx context.Context, tx pgx.Tx, orgID, metaCallID string) (*model.WhatsAppCall, error) {
	row := tx.QueryRow(ctx, sqlWACallByCallID, metaCallID, orgID)
	c, err := scanWACall(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppRepository.GetCallByCallID: %w", err)
	}
	return c, nil
}

// UpdateCallState transitions a call to the given state.
func (r *WhatsAppRepository) UpdateCallState(ctx context.Context, tx pgx.Tx, orgID, callID string, state model.WhatsAppCallState) (*model.WhatsAppCall, error) {
	row := tx.QueryRow(ctx, sqlWACallUpdateState, callID, orgID, state)
	c, err := scanWACall(row)
	if err != nil {
		return nil, fmt.Errorf("WhatsAppRepository.UpdateCallState: %w", err)
	}
	return c, nil
}

// ListCalls returns calls for an org with pagination.
func (r *WhatsAppRepository) ListCalls(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.WhatsAppCall, int, error) {
	var total int
	if err := tx.QueryRow(ctx, sqlWACallCount, orgID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("WhatsAppRepository.ListCalls count: %w", err)
	}

	rows, err := tx.Query(ctx, sqlWACallList, orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("WhatsAppRepository.ListCalls query: %w", err)
	}
	defer rows.Close()

	var calls []model.WhatsAppCall
	for rows.Next() {
		var c model.WhatsAppCall
		if err := rows.Scan(
			&c.ID,
			&c.OrgID,
			&c.CallID,
			&c.PhoneNumberID,
			&c.Direction,
			&c.State,
			&c.Caller,
			&c.Callee,
			&c.StartedAt,
			&c.EndedAt,
			&c.DurationSeconds,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("WhatsAppRepository.ListCalls scan: %w", err)
		}
		calls = append(calls, c)
	}
	return calls, total, rows.Err()
}

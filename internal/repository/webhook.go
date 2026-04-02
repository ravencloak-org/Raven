package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// WebhookRepository handles database operations for webhook configs and deliveries.
// All operations use a pgx.Tx with org_id set for RLS enforcement.
type WebhookRepository struct {
	pool *pgxpool.Pool
}

// NewWebhookRepository creates a new WebhookRepository.
func NewWebhookRepository(pool *pgxpool.Pool) *WebhookRepository {
	return &WebhookRepository{pool: pool}
}

const (
	sqlGetWebhookByID = `SELECT id, org_id, name, url, secret, events,
		COALESCE(headers, '{}') AS headers,
		status, max_retries, last_triggered_at, failure_count,
		COALESCE(created_by::text, '') AS created_by,
		created_at, updated_at
	FROM webhook_configs WHERE id = $1 AND org_id = $2`

	sqlListWebhooks = `SELECT id, org_id, name, url, secret, events,
		COALESCE(headers, '{}') AS headers,
		status, max_retries, last_triggered_at, failure_count,
		COALESCE(created_by::text, '') AS created_by,
		created_at, updated_at
	FROM webhook_configs WHERE org_id = $1 ORDER BY created_at DESC`

	sqlListActiveForEvent = `SELECT id, org_id, name, url, secret, events,
		COALESCE(headers, '{}') AS headers,
		status, max_retries, last_triggered_at, failure_count,
		COALESCE(created_by::text, '') AS created_by,
		created_at, updated_at
	FROM webhook_configs
	WHERE org_id = $1 AND status = 'active' AND $2 = ANY(events)
	ORDER BY created_at ASC`

	sqlCreateWebhook = `INSERT INTO webhook_configs (org_id, name, url, secret, events, headers, max_retries, created_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id, org_id, name, url, secret, events,
		COALESCE(headers, '{}') AS headers,
		status, max_retries, last_triggered_at, failure_count,
		COALESCE(created_by::text, '') AS created_by,
		created_at, updated_at`

	sqlUpdateWebhook = `UPDATE webhook_configs SET
		name = COALESCE($3, name),
		url = COALESCE($4, url),
		secret = COALESCE($5, secret),
		events = COALESCE($6, events),
		headers = COALESCE($7, headers),
		status = COALESCE($8, status),
		max_retries = COALESCE($9, max_retries)
	WHERE id = $1 AND org_id = $2
	RETURNING id, org_id, name, url, secret, events,
		COALESCE(headers, '{}') AS headers,
		status, max_retries, last_triggered_at, failure_count,
		COALESCE(created_by::text, '') AS created_by,
		created_at, updated_at`

	sqlSetWebhookStatus = `UPDATE webhook_configs SET status = $2 WHERE id = $1`
)

func scanWebhookConfig(row pgx.Row) (*model.WebhookConfig, error) {
	var w model.WebhookConfig
	var headersBytes []byte
	var createdBy string
	err := row.Scan(
		&w.ID, &w.OrgID, &w.Name, &w.URL, &w.Secret, &w.Events,
		&headersBytes,
		&w.Status, &w.MaxRetries, &w.LastTriggeredAt, &w.FailureCount,
		&createdBy,
		&w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if createdBy != "" {
		w.CreatedBy = createdBy
	}
	if len(headersBytes) > 2 {
		_ = json.Unmarshal(headersBytes, &w.Headers)
	}
	if w.Events == nil {
		w.Events = []string{}
	}
	return &w, nil
}

// Create inserts a new webhook config within a transaction.
func (r *WebhookRepository) Create(ctx context.Context, tx pgx.Tx, orgID string, req model.CreateWebhookRequest, createdBy string) (*model.WebhookConfig, error) {
	headersBytes, err := json.Marshal(req.Headers)
	if err != nil {
		headersBytes = []byte("{}")
	}
	if req.Headers == nil {
		headersBytes = []byte("{}")
	}

	maxRetries := 5
	if req.MaxRetries != nil {
		maxRetries = *req.MaxRetries
	}

	row := tx.QueryRow(ctx, sqlCreateWebhook,
		orgID, req.Name, req.URL, req.Secret, req.Events, headersBytes, maxRetries, createdBy,
	)
	created, err := scanWebhookConfig(row)
	if err != nil {
		return nil, fmt.Errorf("WebhookRepository.Create: %w", err)
	}
	return created, nil
}

// GetByID fetches a webhook config by ID within an org.
func (r *WebhookRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, id string) (*model.WebhookConfig, error) {
	row := tx.QueryRow(ctx, sqlGetWebhookByID, id, orgID)
	w, err := scanWebhookConfig(row)
	if err != nil {
		return nil, fmt.Errorf("WebhookRepository.GetByID: %w", err)
	}
	return w, nil
}

// List returns all webhook configs for an org.
func (r *WebhookRepository) List(ctx context.Context, tx pgx.Tx, orgID string) ([]model.WebhookConfig, error) {
	rows, err := tx.Query(ctx, sqlListWebhooks, orgID)
	if err != nil {
		return nil, fmt.Errorf("WebhookRepository.List: %w", err)
	}
	defer rows.Close()

	var hooks []model.WebhookConfig
	for rows.Next() {
		var w model.WebhookConfig
		var headersBytes []byte
		var createdBy string
		if err := rows.Scan(
			&w.ID, &w.OrgID, &w.Name, &w.URL, &w.Secret, &w.Events,
			&headersBytes,
			&w.Status, &w.MaxRetries, &w.LastTriggeredAt, &w.FailureCount,
			&createdBy,
			&w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("WebhookRepository.List scan: %w", err)
		}
		if createdBy != "" {
			w.CreatedBy = createdBy
		}
		if len(headersBytes) > 2 {
			_ = json.Unmarshal(headersBytes, &w.Headers)
		}
		if w.Events == nil {
			w.Events = []string{}
		}
		hooks = append(hooks, w)
	}
	return hooks, rows.Err()
}

// Update applies partial updates to a webhook config.
func (r *WebhookRepository) Update(ctx context.Context, tx pgx.Tx, orgID, id string, req model.UpdateWebhookRequest) (*model.WebhookConfig, error) {
	var headersBytes []byte
	if req.Headers != nil {
		b, err := json.Marshal(req.Headers)
		if err == nil {
			headersBytes = b
		}
	}

	row := tx.QueryRow(ctx, sqlUpdateWebhook,
		id, orgID,
		req.Name, req.URL, req.Secret, req.Events, headersBytes, req.Status, req.MaxRetries,
	)
	w, err := scanWebhookConfig(row)
	if err != nil {
		return nil, fmt.Errorf("WebhookRepository.Update: %w", err)
	}
	return w, nil
}

// Delete removes a webhook config by ID.
func (r *WebhookRepository) Delete(ctx context.Context, tx pgx.Tx, orgID, id string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM webhook_configs WHERE id = $1 AND org_id = $2`,
		id, orgID,
	)
	if err != nil {
		return fmt.Errorf("WebhookRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("WebhookRepository.Delete: webhook %s not found", id)
	}
	return nil
}

// ListActiveForEvent returns all active webhooks subscribed to a given event type.
func (r *WebhookRepository) ListActiveForEvent(ctx context.Context, tx pgx.Tx, orgID, eventType string) ([]model.WebhookConfig, error) {
	rows, err := tx.Query(ctx, sqlListActiveForEvent, orgID, eventType)
	if err != nil {
		return nil, fmt.Errorf("WebhookRepository.ListActiveForEvent: %w", err)
	}
	defer rows.Close()

	var hooks []model.WebhookConfig
	for rows.Next() {
		var w model.WebhookConfig
		var headersBytes []byte
		var createdBy string
		if err := rows.Scan(
			&w.ID, &w.OrgID, &w.Name, &w.URL, &w.Secret, &w.Events,
			&headersBytes,
			&w.Status, &w.MaxRetries, &w.LastTriggeredAt, &w.FailureCount,
			&createdBy,
			&w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("WebhookRepository.ListActiveForEvent scan: %w", err)
		}
		if createdBy != "" {
			w.CreatedBy = createdBy
		}
		if len(headersBytes) > 2 {
			_ = json.Unmarshal(headersBytes, &w.Headers)
		}
		if w.Events == nil {
			w.Events = []string{}
		}
		hooks = append(hooks, w)
	}
	return hooks, rows.Err()
}

// CreateDelivery inserts a new webhook delivery record.
func (r *WebhookRepository) CreateDelivery(ctx context.Context, tx pgx.Tx, d model.WebhookDelivery) error {
	payloadBytes, err := json.Marshal(d.Payload)
	if err != nil {
		return fmt.Errorf("WebhookRepository.CreateDelivery: marshal payload: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO webhook_deliveries (id, webhook_id, org_id, event_type, payload, status, attempt)
		VALUES ($1, $2, $3, $4, $5, 'pending', 0)`,
		d.ID, d.WebhookID, d.OrgID, d.EventType, payloadBytes,
	)
	if err != nil {
		return fmt.Errorf("WebhookRepository.CreateDelivery: %w", err)
	}
	return nil
}

// UpdateDelivery updates a delivery record after an attempt.
func (r *WebhookRepository) UpdateDelivery(ctx context.Context, tx pgx.Tx, id string, success bool, status int, body string) error {
	deliveryStatus := "failed"
	if success {
		deliveryStatus = "delivered"
	}
	_, err := tx.Exec(ctx,
		`UPDATE webhook_deliveries SET
			status = $2,
			response_status = $3,
			response_body = $4,
			attempt = attempt + 1,
			completed_at = CASE WHEN $5 THEN NOW() ELSE NULL END
		WHERE id = $1`,
		id, deliveryStatus, status, body, success,
	)
	if err != nil {
		return fmt.Errorf("WebhookRepository.UpdateDelivery: %w", err)
	}
	return nil
}

// SetWebhookStatus updates the status of a webhook config by ID.
func (r *WebhookRepository) SetWebhookStatus(ctx context.Context, tx pgx.Tx, id string, status model.WebhookStatus) error {
	_, err := tx.Exec(ctx, sqlSetWebhookStatus, id, string(status))
	if err != nil {
		return fmt.Errorf("WebhookRepository.SetWebhookStatus: %w", err)
	}
	return nil
}

// IncrementFailureCount increments the failure_count on a webhook config.
func (r *WebhookRepository) IncrementFailureCount(ctx context.Context, tx pgx.Tx, id string) error {
	_, err := tx.Exec(ctx,
		`UPDATE webhook_configs SET failure_count = failure_count + 1 WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("WebhookRepository.IncrementFailureCount: %w", err)
	}
	return nil
}

// ListDeliveries returns the most recent deliveries for a given webhook.
func (r *WebhookRepository) ListDeliveries(ctx context.Context, tx pgx.Tx, orgID, webhookID string, limit int) ([]model.WebhookDelivery, error) {
	rows, err := tx.Query(ctx,
		`SELECT id, webhook_id, org_id, event_type, payload,
			response_status, COALESCE(response_body, '') AS response_body,
			attempt, status, completed_at, created_at
		FROM webhook_deliveries
		WHERE org_id = $1 AND webhook_id = $2
		ORDER BY created_at DESC LIMIT $3`,
		orgID, webhookID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("WebhookRepository.ListDeliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []model.WebhookDelivery
	for rows.Next() {
		var d model.WebhookDelivery
		var payloadBytes []byte
		var deliveryStatus string
		if err := rows.Scan(
			&d.ID, &d.WebhookID, &d.OrgID, &d.EventType, &payloadBytes,
			&d.ResponseStatus, &d.ResponseBody,
			&d.AttemptCount, &deliveryStatus, &d.DeliveredAt, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("WebhookRepository.ListDeliveries scan: %w", err)
		}
		d.Success = deliveryStatus == "delivered"
		if len(payloadBytes) > 0 {
			_ = json.Unmarshal(payloadBytes, &d.Payload)
		}
		deliveries = append(deliveries, d)
	}
	return deliveries, rows.Err()
}

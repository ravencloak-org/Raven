package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravencloak-org/Raven/internal/model"
)

// AirbyteRepository handles database operations for Airbyte connectors and sync history.
// All operations use a pgx.Tx with org_id set for RLS enforcement.
type AirbyteRepository struct {
	pool *pgxpool.Pool
}

// NewAirbyteRepository creates a new AirbyteRepository.
func NewAirbyteRepository(pool *pgxpool.Pool) *AirbyteRepository {
	return &AirbyteRepository{pool: pool}
}

const connectorColumns = `id, org_id, knowledge_base_id, name, connector_type,
	COALESCE(config, '{}') AS config, sync_mode,
	schedule_cron, status, last_sync_at,
	last_sync_status, last_sync_records,
	COALESCE(created_by::text, '') AS created_by,
	created_at, updated_at`

func scanConnector(row pgx.Row) (*model.AirbyteConnector, error) {
	var c model.AirbyteConnector
	var createdBy string
	err := row.Scan(
		&c.ID,
		&c.OrgID,
		&c.KnowledgeBaseID,
		&c.Name,
		&c.ConnectorType,
		&c.Config,
		&c.SyncMode,
		&c.ScheduleCron,
		&c.Status,
		&c.LastSyncAt,
		&c.LastSyncStatus,
		&c.LastSyncRecords,
		&createdBy,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if createdBy != "" {
		c.CreatedBy = &createdBy
	}
	return &c, nil
}

// Create inserts a new Airbyte connector within a transaction.
func (r *AirbyteRepository) Create(ctx context.Context, tx pgx.Tx, orgID string, req model.CreateConnectorRequest, createdBy string) (*model.AirbyteConnector, error) {
	syncMode := model.SyncModeFullRefresh
	if req.SyncMode != "" {
		syncMode = req.SyncMode
	}

	row := tx.QueryRow(ctx,
		`INSERT INTO airbyte_connectors (org_id, knowledge_base_id, name, connector_type, config, sync_mode, schedule_cron, created_by)
		 VALUES ($1, $2, $3, $4, COALESCE($5::jsonb, '{}'), $6, $7, NULLIF($8, '')::uuid)
		 RETURNING `+connectorColumns,
		orgID, req.KnowledgeBaseID, req.Name, req.ConnectorType, req.Config, syncMode, req.ScheduleCron, createdBy,
	)
	c, err := scanConnector(row)
	if err != nil {
		return nil, fmt.Errorf("AirbyteRepository.Create: %w", err)
	}
	return c, nil
}

// GetByID fetches a connector by its primary key within an org.
func (r *AirbyteRepository) GetByID(ctx context.Context, tx pgx.Tx, orgID, connectorID string) (*model.AirbyteConnector, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+connectorColumns+`
		 FROM airbyte_connectors
		 WHERE id = $1 AND org_id = $2`,
		connectorID, orgID,
	)
	c, err := scanConnector(row)
	if err != nil {
		return nil, fmt.Errorf("AirbyteRepository.GetByID: %w", err)
	}
	return c, nil
}

// List returns a paginated list of connectors for an organisation.
func (r *AirbyteRepository) List(ctx context.Context, tx pgx.Tx, orgID string, page, pageSize int) ([]model.AirbyteConnector, int, error) {
	var total int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM airbyte_connectors WHERE org_id = $1`,
		orgID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("AirbyteRepository.List count: %w", err)
	}

	offset := (page - 1) * pageSize
	rows, err := tx.Query(ctx,
		`SELECT `+connectorColumns+`
		 FROM airbyte_connectors
		 WHERE org_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		orgID, pageSize, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("AirbyteRepository.List query: %w", err)
	}
	defer rows.Close()

	var connectors []model.AirbyteConnector
	for rows.Next() {
		var c model.AirbyteConnector
		var createdBy string
		if err := rows.Scan(
			&c.ID, &c.OrgID, &c.KnowledgeBaseID, &c.Name, &c.ConnectorType,
			&c.Config, &c.SyncMode, &c.ScheduleCron, &c.Status,
			&c.LastSyncAt, &c.LastSyncStatus, &c.LastSyncRecords,
			&createdBy, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("AirbyteRepository.List scan: %w", err)
		}
		if createdBy != "" {
			c.CreatedBy = &createdBy
		}
		connectors = append(connectors, c)
	}
	return connectors, total, rows.Err()
}

// Update applies partial updates to a connector.
func (r *AirbyteRepository) Update(ctx context.Context, tx pgx.Tx, orgID, connectorID string, req model.UpdateConnectorRequest) (*model.AirbyteConnector, error) {
	row := tx.QueryRow(ctx,
		`UPDATE airbyte_connectors
		 SET
		   name          = COALESCE($3, name),
		   config        = CASE WHEN $4::jsonb IS NOT NULL THEN $4::jsonb ELSE config END,
		   sync_mode     = COALESCE($5, sync_mode),
		   schedule_cron = COALESCE($6, schedule_cron),
		   status        = COALESCE($7, status),
		   updated_at    = NOW()
		 WHERE id = $1 AND org_id = $2
		 RETURNING `+connectorColumns,
		connectorID, orgID, req.Name, req.Config, req.SyncMode, req.ScheduleCron, req.Status,
	)
	c, err := scanConnector(row)
	if err != nil {
		return nil, fmt.Errorf("AirbyteRepository.Update: %w", err)
	}
	return c, nil
}

// Delete permanently removes a connector.
func (r *AirbyteRepository) Delete(ctx context.Context, tx pgx.Tx, orgID, connectorID string) error {
	tag, err := tx.Exec(ctx,
		`DELETE FROM airbyte_connectors WHERE id = $1 AND org_id = $2`,
		connectorID, orgID,
	)
	if err != nil {
		return fmt.Errorf("AirbyteRepository.Delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("AirbyteRepository.Delete: connector %s not found", connectorID)
	}
	return nil
}

// --- Sync history operations ---

const syncHistoryColumns = `id, connector_id, org_id, status, records_synced, records_failed,
	bytes_synced, started_at, completed_at, COALESCE(error_message, '') AS error_message`

func scanSyncHistory(row pgx.Row) (*model.SyncHistory, error) {
	var h model.SyncHistory
	err := row.Scan(
		&h.ID,
		&h.ConnectorID,
		&h.OrgID,
		&h.Status,
		&h.RecordsSynced,
		&h.RecordsFailed,
		&h.BytesSynced,
		&h.StartedAt,
		&h.CompletedAt,
		&h.ErrorMessage,
	)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// ListSyncHistory returns recent sync history records for a connector.
func (r *AirbyteRepository) ListSyncHistory(ctx context.Context, tx pgx.Tx, connectorID, orgID string, limit int) ([]model.SyncHistory, error) {
	rows, err := tx.Query(ctx,
		`SELECT `+syncHistoryColumns+`
		 FROM airbyte_sync_history
		 WHERE connector_id = $1 AND org_id = $2
		 ORDER BY started_at DESC
		 LIMIT $3`,
		connectorID, orgID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("AirbyteRepository.ListSyncHistory: %w", err)
	}
	defer rows.Close()

	var history []model.SyncHistory
	for rows.Next() {
		var h model.SyncHistory
		if err := rows.Scan(
			&h.ID, &h.ConnectorID, &h.OrgID, &h.Status,
			&h.RecordsSynced, &h.RecordsFailed, &h.BytesSynced,
			&h.StartedAt, &h.CompletedAt, &h.ErrorMessage,
		); err != nil {
			return nil, fmt.Errorf("AirbyteRepository.ListSyncHistory scan: %w", err)
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

// CreateSyncRun inserts a new sync history record with status "running".
func (r *AirbyteRepository) CreateSyncRun(ctx context.Context, tx pgx.Tx, connectorID, orgID string) (*model.SyncHistory, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO airbyte_sync_history (connector_id, org_id, status)
		 VALUES ($1, $2, 'running')
		 RETURNING `+syncHistoryColumns,
		connectorID, orgID,
	)
	h, err := scanSyncHistory(row)
	if err != nil {
		return nil, fmt.Errorf("AirbyteRepository.CreateSyncRun: %w", err)
	}
	return h, nil
}

// CompleteSyncRun updates a sync history record with final status and metrics.
func (r *AirbyteRepository) CompleteSyncRun(ctx context.Context, tx pgx.Tx, syncID, orgID, status string, recordsSynced, recordsFailed int, errMsg string) error {
	tag, err := tx.Exec(ctx,
		`UPDATE airbyte_sync_history
		 SET status = $3, records_synced = $4, records_failed = $5,
		     error_message = NULLIF($6, ''), completed_at = NOW()
		 WHERE id = $1 AND org_id = $2`,
		syncID, orgID, status, recordsSynced, recordsFailed, errMsg,
	)
	if err != nil {
		return fmt.Errorf("AirbyteRepository.CompleteSyncRun: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("AirbyteRepository.CompleteSyncRun: sync run %s not found", syncID)
	}
	return nil
}

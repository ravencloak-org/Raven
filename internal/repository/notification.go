package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
)

// NotificationRepository handles database operations for notification configs and logs.
type NotificationRepository struct {
	pool *pgxpool.Pool
}

// NewNotificationRepository creates a new NotificationRepository.
func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

const (
	sqlCreateNotificationConfig = `INSERT INTO notification_configs (org_id, notification_type, recipients, enabled, config)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, org_id, notification_type,
				COALESCE(recipients, '{}') AS recipients,
				enabled,
				COALESCE(config, '{}') AS config,
				created_at, updated_at`

	sqlGetNotificationConfig = `SELECT id, org_id, notification_type,
			COALESCE(recipients, '{}') AS recipients,
			enabled,
			COALESCE(config, '{}') AS config,
			created_at, updated_at
		FROM notification_configs WHERE id = $1 AND org_id = $2`

	sqlListNotificationConfigs = `SELECT id, org_id, notification_type,
			COALESCE(recipients, '{}') AS recipients,
			enabled,
			COALESCE(config, '{}') AS config,
			created_at, updated_at
		FROM notification_configs WHERE org_id = $1 ORDER BY created_at ASC`

	sqlUpdateNotificationConfig = `UPDATE notification_configs SET
			recipients = COALESCE($3, recipients),
			enabled    = COALESCE($4, enabled),
			config     = COALESCE($5, config)
		WHERE id = $1 AND org_id = $2
		RETURNING id, org_id, notification_type,
			COALESCE(recipients, '{}') AS recipients,
			enabled,
			COALESCE(config, '{}') AS config,
			created_at, updated_at`

	sqlListEnabledByType = `SELECT id, org_id, notification_type,
			COALESCE(recipients, '{}') AS recipients,
			enabled,
			COALESCE(config, '{}') AS config,
			created_at, updated_at
		FROM notification_configs
		WHERE org_id = $1 AND notification_type = $2 AND enabled = true
		ORDER BY created_at ASC`
)

func scanNotificationConfig(row pgx.Row) (*model.NotificationConfig, error) {
	var c model.NotificationConfig
	var configBytes []byte
	err := row.Scan(
		&c.ID, &c.OrgID, &c.NotificationType,
		&c.Recipients,
		&c.Enabled,
		&configBytes,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if c.Recipients == nil {
		c.Recipients = []string{}
	}
	if len(configBytes) > 2 {
		_ = json.Unmarshal(configBytes, &c.Config)
	}
	if c.Config == nil {
		c.Config = map[string]any{}
	}
	return &c, nil
}

// CreateConfig inserts a new notification config for an org.
func (r *NotificationRepository) CreateConfig(ctx context.Context, orgID string, req model.CreateNotificationConfigRequest) (*model.NotificationConfig, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	configBytes, err := json.Marshal(req.Config)
	if err != nil {
		configBytes = []byte("{}")
	}
	if req.Recipients == nil {
		req.Recipients = []string{}
	}

	var created *model.NotificationConfig
	err = db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, sqlCreateNotificationConfig,
			orgID, req.NotificationType, req.Recipients, enabled, configBytes,
		)
		var e error
		created, e = scanNotificationConfig(row)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("NotificationRepository.CreateConfig: %w", err)
	}
	return created, nil
}

// GetConfig fetches a notification config by ID within an org.
func (r *NotificationRepository) GetConfig(ctx context.Context, orgID, id string) (*model.NotificationConfig, error) {
	var cfg *model.NotificationConfig
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, sqlGetNotificationConfig, id, orgID)
		var e error
		cfg, e = scanNotificationConfig(row)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("NotificationRepository.GetConfig: %w", err)
	}
	return cfg, nil
}

// ListConfigs returns all notification configs for an org.
func (r *NotificationRepository) ListConfigs(ctx context.Context, orgID string) ([]model.NotificationConfig, error) {
	var configs []model.NotificationConfig
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sqlListNotificationConfigs, orgID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var c model.NotificationConfig
			var configBytes []byte
			if err := rows.Scan(
				&c.ID, &c.OrgID, &c.NotificationType,
				&c.Recipients,
				&c.Enabled,
				&configBytes,
				&c.CreatedAt, &c.UpdatedAt,
			); err != nil {
				return fmt.Errorf("scan: %w", err)
			}
			if c.Recipients == nil {
				c.Recipients = []string{}
			}
			if len(configBytes) > 2 {
				_ = json.Unmarshal(configBytes, &c.Config)
			}
			if c.Config == nil {
				c.Config = map[string]any{}
			}
			configs = append(configs, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("NotificationRepository.ListConfigs: %w", err)
	}
	return configs, nil
}

// UpdateConfig applies partial updates to a notification config.
func (r *NotificationRepository) UpdateConfig(ctx context.Context, orgID, id string, req model.UpdateNotificationConfigRequest) (*model.NotificationConfig, error) {
	var configBytes []byte
	if req.Config != nil {
		b, err := json.Marshal(req.Config)
		if err == nil {
			configBytes = b
		}
	}

	var updated *model.NotificationConfig
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, sqlUpdateNotificationConfig,
			id, orgID,
			req.Recipients, req.Enabled, configBytes,
		)
		var e error
		updated, e = scanNotificationConfig(row)
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("NotificationRepository.UpdateConfig: %w", err)
	}
	return updated, nil
}

// DeleteConfig removes a notification config by ID.
func (r *NotificationRepository) DeleteConfig(ctx context.Context, orgID, id string) error {
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`DELETE FROM notification_configs WHERE id = $1 AND org_id = $2`,
			id, orgID,
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("config %s not found", id)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("NotificationRepository.DeleteConfig: %w", err)
	}
	return nil
}

// ListEnabledByType returns all enabled notification configs for an org and type.
func (r *NotificationRepository) ListEnabledByType(ctx context.Context, orgID string, notifType model.NotificationType) ([]model.NotificationConfig, error) {
	var configs []model.NotificationConfig
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, sqlListEnabledByType, orgID, notifType)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var c model.NotificationConfig
			var configBytes []byte
			if err := rows.Scan(
				&c.ID, &c.OrgID, &c.NotificationType,
				&c.Recipients,
				&c.Enabled,
				&configBytes,
				&c.CreatedAt, &c.UpdatedAt,
			); err != nil {
				return fmt.Errorf("scan: %w", err)
			}
			if c.Recipients == nil {
				c.Recipients = []string{}
			}
			if len(configBytes) > 2 {
				_ = json.Unmarshal(configBytes, &c.Config)
			}
			if c.Config == nil {
				c.Config = map[string]any{}
			}
			configs = append(configs, c)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("NotificationRepository.ListEnabledByType: %w", err)
	}
	return configs, nil
}

// CreateLog inserts a notification delivery log entry.
func (r *NotificationRepository) CreateLog(ctx context.Context, log model.NotificationLog) error {
	err := db.WithOrgID(ctx, r.pool, log.OrgID, func(tx pgx.Tx) error {
		var sentAt any
		if log.SentAt != nil {
			sentAt = *log.SentAt
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO notification_log
				(org_id, config_id, notification_type, recipient, subject, status, error_message, sent_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			log.OrgID, log.ConfigID, log.NotificationType,
			log.Recipient, log.Subject, log.Status,
			log.ErrorMessage, sentAt,
		)
		return err
	})
	if err != nil {
		return fmt.Errorf("NotificationRepository.CreateLog: %w", err)
	}
	return nil
}

// ListLogs returns the most recent notification log entries for an org.
func (r *NotificationRepository) ListLogs(ctx context.Context, orgID string, limit int) ([]model.NotificationLog, error) {
	if limit <= 0 {
		limit = 50
	}

	var logs []model.NotificationLog
	err := db.WithOrgID(ctx, r.pool, orgID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id, org_id, config_id, notification_type, recipient, subject,
				status, error_message, sent_at, created_at
			FROM notification_log
			WHERE org_id = $1
			ORDER BY created_at DESC
			LIMIT $2`,
			orgID, limit,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l model.NotificationLog
			var sentAt *time.Time
			if err := rows.Scan(
				&l.ID, &l.OrgID, &l.ConfigID, &l.NotificationType,
				&l.Recipient, &l.Subject,
				&l.Status, &l.ErrorMessage, &sentAt, &l.CreatedAt,
			); err != nil {
				return fmt.Errorf("scan: %w", err)
			}
			l.SentAt = sentAt
			logs = append(logs, l)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("NotificationRepository.ListLogs: %w", err)
	}
	return logs, nil
}

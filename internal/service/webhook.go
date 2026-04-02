package service

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// WebhookService contains business logic for webhook management and dispatch.
type WebhookService struct {
	repo        *repository.WebhookRepository
	pool        *pgxpool.Pool
	queueClient *queue.Client
}

// NewWebhookService creates a new WebhookService.
func NewWebhookService(repo *repository.WebhookRepository, pool *pgxpool.Pool, queueClient *queue.Client) *WebhookService {
	return &WebhookService{repo: repo, pool: pool, queueClient: queueClient}
}

// Create validates and creates a new webhook config.
func (s *WebhookService) Create(ctx context.Context, orgID, userID string, req model.CreateWebhookRequest) (*model.WebhookConfig, error) {
	var created *model.WebhookConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		created, e = s.repo.Create(ctx, tx, orgID, req, userID)
		return e
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apierror.NewBadRequest("a webhook with this name already exists")
		}
		return nil, apierror.NewInternal("failed to create webhook: " + err.Error())
	}
	return created, nil
}

// GetByID retrieves a webhook config by ID.
func (s *WebhookService) GetByID(ctx context.Context, orgID, id string) (*model.WebhookConfig, error) {
	var hook *model.WebhookConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		hook, e = s.repo.GetByID(ctx, tx, orgID, id)
		return e
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("webhook not found")
		}
		return nil, apierror.NewInternal("failed to fetch webhook: " + err.Error())
	}
	return hook, nil
}

// List returns all webhook configs for an org.
func (s *WebhookService) List(ctx context.Context, orgID string) ([]model.WebhookConfig, error) {
	var hooks []model.WebhookConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		hooks, e = s.repo.List(ctx, tx, orgID)
		return e
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list webhooks: " + err.Error())
	}
	return hooks, nil
}

// Update applies partial updates to a webhook config.
func (s *WebhookService) Update(ctx context.Context, orgID, id string, req model.UpdateWebhookRequest) (*model.WebhookConfig, error) {
	var hook *model.WebhookConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		hook, e = s.repo.Update(ctx, tx, orgID, id, req)
		return e
	})
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("webhook not found")
		}
		return nil, apierror.NewInternal("failed to update webhook: " + err.Error())
	}
	return hook, nil
}

// Delete removes a webhook config.
func (s *WebhookService) Delete(ctx context.Context, orgID, id string) error {
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		return s.repo.Delete(ctx, tx, orgID, id)
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("webhook not found")
		}
		return apierror.NewInternal("failed to delete webhook: " + err.Error())
	}
	return nil
}

// ListDeliveries returns recent delivery attempts for a webhook.
func (s *WebhookService) ListDeliveries(ctx context.Context, orgID, webhookID string, limit int) ([]model.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 50
	}
	var deliveries []model.WebhookDelivery
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		deliveries, e = s.repo.ListDeliveries(ctx, tx, orgID, webhookID, limit)
		return e
	})
	if err != nil {
		return nil, apierror.NewInternal("failed to list webhook deliveries: " + err.Error())
	}
	if deliveries == nil {
		deliveries = []model.WebhookDelivery{}
	}
	return deliveries, nil
}

// Dispatch looks up active webhooks for an event and enqueues an Asynq delivery job for each.
func (s *WebhookService) Dispatch(ctx context.Context, orgID, eventType string, payload map[string]any) error {
	var hooks []model.WebhookConfig
	err := db.WithOrgID(ctx, s.pool, orgID, func(tx pgx.Tx) error {
		var e error
		hooks, e = s.repo.ListActiveForEvent(ctx, tx, orgID, eventType)
		return e
	})
	if err != nil {
		return err
	}
	for _, h := range hooks {
		p := queue.WebhookDeliveryPayload{
			WebhookID: h.ID,
			OrgID:     orgID,
			EventType: eventType,
			Payload:   payload,
		}
		if enqErr := s.queueClient.EnqueueWebhookDelivery(ctx, p); enqErr != nil {
			// Log but don't fail dispatch if one enqueue fails.
			_ = enqErr
		}
	}
	return nil
}

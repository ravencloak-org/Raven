package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// NotificationService contains business logic for notification config management
// and async email delivery via Asynq.
type NotificationService struct {
	repo        *repository.NotificationRepository
	queueClient *queue.Client
}

// NewNotificationService creates a new NotificationService.
func NewNotificationService(repo *repository.NotificationRepository, queueClient *queue.Client) *NotificationService {
	return &NotificationService{repo: repo, queueClient: queueClient}
}

// CreateConfig validates and creates a notification config.
func (s *NotificationService) CreateConfig(ctx context.Context, orgID string, req model.CreateNotificationConfigRequest) (*model.NotificationConfig, error) {
	switch req.NotificationType {
	case model.NotificationTypeConversationSummary, model.NotificationTypeAdminDigest, model.NotificationTypeCustom:
		// valid
	default:
		return nil, apierror.NewBadRequest(fmt.Sprintf("invalid notification_type: %s", req.NotificationType))
	}
	if len(req.Recipients) == 0 {
		return nil, apierror.NewBadRequest("recipients must not be empty")
	}

	cfg, err := s.repo.CreateConfig(ctx, orgID, req)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, apierror.NewBadRequest("a notification config with this type already exists")
		}
		return nil, apierror.NewInternal("failed to create notification config: " + err.Error())
	}
	return cfg, nil
}

// GetConfig retrieves a notification config by ID.
func (s *NotificationService) GetConfig(ctx context.Context, orgID, id string) (*model.NotificationConfig, error) {
	cfg, err := s.repo.GetConfig(ctx, orgID, id)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("notification config not found")
		}
		return nil, apierror.NewInternal("failed to fetch notification config: " + err.Error())
	}
	return cfg, nil
}

// ListConfigs returns all notification configs for an org.
func (s *NotificationService) ListConfigs(ctx context.Context, orgID string) ([]model.NotificationConfig, error) {
	configs, err := s.repo.ListConfigs(ctx, orgID)
	if err != nil {
		return nil, apierror.NewInternal("failed to list notification configs: " + err.Error())
	}
	if configs == nil {
		configs = []model.NotificationConfig{}
	}
	return configs, nil
}

// UpdateConfig applies partial updates to a notification config.
func (s *NotificationService) UpdateConfig(ctx context.Context, orgID, id string, req model.UpdateNotificationConfigRequest) (*model.NotificationConfig, error) {
	cfg, err := s.repo.UpdateConfig(ctx, orgID, id, req)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, apierror.NewNotFound("notification config not found")
		}
		return nil, apierror.NewInternal("failed to update notification config: " + err.Error())
	}
	return cfg, nil
}

// DeleteConfig removes a notification config.
func (s *NotificationService) DeleteConfig(ctx context.Context, orgID, id string) error {
	if err := s.repo.DeleteConfig(ctx, orgID, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return apierror.NewNotFound("notification config not found")
		}
		return apierror.NewInternal("failed to delete notification config: " + err.Error())
	}
	return nil
}

// ListLogs returns recent notification log entries for an org.
func (s *NotificationService) ListLogs(ctx context.Context, orgID string, limit int) ([]model.NotificationLog, error) {
	logs, err := s.repo.ListLogs(ctx, orgID, limit)
	if err != nil {
		return nil, apierror.NewInternal("failed to list notification logs: " + err.Error())
	}
	if logs == nil {
		logs = []model.NotificationLog{}
	}
	return logs, nil
}

// TriggerConversationSummary enqueues send-email Asynq jobs for all enabled
// conversation_summary configs in the given org.
func (s *NotificationService) TriggerConversationSummary(ctx context.Context, orgID, sessionID string, summary string) error {
	configs, err := s.repo.ListEnabledByType(ctx, orgID, model.NotificationTypeConversationSummary)
	if err != nil {
		return fmt.Errorf("list enabled configs: %w", err)
	}

	subject := fmt.Sprintf("Conversation Summary — Session %s", sessionID)

	for _, cfg := range configs {
		payload := model.SendEmailPayload{
			OrgID:            orgID,
			ConfigID:         cfg.ID,
			NotificationType: model.NotificationTypeConversationSummary,
			Recipients:       cfg.Recipients,
			Subject:          subject,
			Body:             summary,
		}
		if err := s.queueClient.EnqueueSendEmail(ctx, payload); err != nil {
			return fmt.Errorf("enqueue send-email for config %s: %w", cfg.ID, err)
		}
	}
	return nil
}

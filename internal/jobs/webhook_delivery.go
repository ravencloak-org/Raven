package jobs

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
)

// TypeWebhookDelivery is the Asynq task type for delivering webhook events.
const TypeWebhookDelivery = queue.TypeWebhookDelivery

// WebhookDeliveryHandler processes webhook delivery tasks.
type WebhookDeliveryHandler struct {
	pool       *pgxpool.Pool
	repo       *repository.WebhookRepository
	httpClient *http.Client
	logger     *slog.Logger
}

// NewWebhookDeliveryHandler creates a new WebhookDeliveryHandler.
func NewWebhookDeliveryHandler(pool *pgxpool.Pool, repo *repository.WebhookRepository, logger *slog.Logger) *WebhookDeliveryHandler {
	return &WebhookDeliveryHandler{
		pool: pool,
		repo: repo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// ProcessTask implements asynq.Handler for webhook delivery tasks.
func (h *WebhookDeliveryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var p queue.WebhookDeliveryPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal WebhookDeliveryPayload: %w", err)
	}

	h.logger.Info("delivering webhook",
		"webhook_id", p.WebhookID,
		"org_id", p.OrgID,
		"event_type", p.EventType,
	)

	// Fetch webhook config to get the secret and URL.
	var hook *model.WebhookConfig
	err := db.WithOrgID(ctx, h.pool, p.OrgID, func(tx pgx.Tx) error {
		var e error
		hook, e = h.repo.GetByID(ctx, tx, p.OrgID, p.WebhookID)
		return e
	})
	if err != nil {
		return fmt.Errorf("get webhook config: %w", err)
	}

	// Build the request body.
	body := map[string]any{
		"event_type": p.EventType,
		"org_id":     p.OrgID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"data":       p.Payload,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal webhook body: %w", err)
	}

	// Compute HMAC-SHA256 signature.
	mac := hmac.New(sha256.New, []byte(hook.Secret))
	mac.Write(bodyBytes)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Build and send the HTTP request.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Raven-Signature", signature)
	req.Header.Set("X-Raven-Event", p.EventType)
	for k, v := range hook.Headers {
		req.Header.Set(k, v)
	}

	resp, err := h.httpClient.Do(req)

	var responseStatus int
	var responseBody string
	success := false

	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		responseStatus = resp.StatusCode
		if rb, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096)); readErr == nil {
			responseBody = string(rb)
		}
		success = resp.StatusCode >= 200 && resp.StatusCode < 300
	} else {
		responseBody = err.Error()
	}

	// Record the delivery attempt using the specific delivery record ID.
	dbErr := db.WithOrgID(ctx, h.pool, p.OrgID, func(tx pgx.Tx) error {
		return h.repo.UpdateDelivery(ctx, tx, p.DeliveryID, success, responseStatus, responseBody)
	})
	if dbErr != nil {
		h.logger.Warn("webhook: failed to record delivery",
			slog.String("delivery_id", p.DeliveryID),
			slog.String("webhook_id", p.WebhookID),
			slog.String("error", dbErr.Error()),
		)
	}

	if !success {
		// Increment failure count and check against max_retries.
		var updatedHook *model.WebhookConfig
		incrErr := db.WithOrgID(ctx, h.pool, p.OrgID, func(tx pgx.Tx) error {
			if e := h.repo.IncrementFailureCount(ctx, tx, p.WebhookID); e != nil {
				return e
			}
			var e error
			updatedHook, e = h.repo.GetByID(ctx, tx, p.OrgID, p.WebhookID)
			return e
		})
		if incrErr != nil {
			h.logger.Warn("webhook: failed to increment failure count",
				slog.String("webhook_id", p.WebhookID),
				slog.String("error", incrErr.Error()),
			)
		}

		// If failure_count has reached max_retries, mark the webhook as failed and stop retrying.
		if updatedHook != nil && updatedHook.FailureCount >= updatedHook.MaxRetries {
			disableErr := db.WithOrgID(ctx, h.pool, p.OrgID, func(tx pgx.Tx) error {
				return h.repo.SetWebhookStatus(ctx, tx, p.WebhookID, model.WebhookStatusFailed)
			})
			if disableErr != nil {
				h.logger.Warn("webhook: failed to mark webhook as failed",
					slog.String("webhook_id", p.WebhookID),
					slog.String("error", disableErr.Error()),
				)
			} else {
				h.logger.Warn("webhook: max retries reached, marking webhook as failed",
					slog.String("webhook_id", p.WebhookID),
					slog.Int("failure_count", updatedHook.FailureCount),
					slog.Int("max_retries", updatedHook.MaxRetries),
				)
			}
			if err != nil {
				return fmt.Errorf("%w: webhook delivery failed (HTTP error): %w", asynq.SkipRetry, err)
			}
			return fmt.Errorf("%w: webhook delivery failed: HTTP %d", asynq.SkipRetry, responseStatus)
		}

		if err != nil {
			return fmt.Errorf("webhook delivery failed (HTTP error): %w", err)
		}
		return fmt.Errorf("webhook delivery failed: HTTP %d", responseStatus)
	}

	h.logger.Info("webhook delivered",
		"webhook_id", p.WebhookID,
		"status", responseStatus,
	)

	return nil
}

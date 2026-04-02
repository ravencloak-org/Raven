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
		defer resp.Body.Close()
		responseStatus = resp.StatusCode
		if rb, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096)); readErr == nil {
			responseBody = string(rb)
		}
		success = resp.StatusCode >= 200 && resp.StatusCode < 300
	} else {
		responseBody = err.Error()
	}

	// Record the delivery attempt.
	dbErr := db.WithOrgID(ctx, h.pool, p.OrgID, func(tx pgx.Tx) error {
		return h.repo.UpdateDelivery(ctx, tx, p.WebhookID, success, responseStatus, responseBody)
	})
	if dbErr != nil {
		h.logger.Warn("webhook: failed to record delivery",
			slog.String("webhook_id", p.WebhookID),
			slog.String("error", dbErr.Error()),
		)
	}

	if !success {
		// Increment failure count on the webhook config.
		incrErr := db.WithOrgID(ctx, h.pool, p.OrgID, func(tx pgx.Tx) error {
			return h.repo.IncrementFailureCount(ctx, tx, p.WebhookID)
		})
		if incrErr != nil {
			h.logger.Warn("webhook: failed to increment failure count",
				slog.String("webhook_id", p.WebhookID),
				slog.String("error", incrErr.Error()),
			)
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

package jobs

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravencloak-org/Raven/internal/db"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/queue"
	"github.com/ravencloak-org/Raven/internal/repository"
)

// errPrivateIP is returned when a webhook URL resolves to a private/reserved IP.
var errPrivateIP = errors.New("webhook URL resolves to a private or reserved IP address")

// privateIPNets contains CIDR ranges that must be blocked for SSRF prevention.
var privateIPNets []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC 1918
		"172.16.0.0/12",  // RFC 1918
		"192.168.0.0/16", // RFC 1918
		"169.254.0.0/16", // link-local
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	} {
		_, ipNet, _ := net.ParseCIDR(cidr)
		privateIPNets = append(privateIPNets, ipNet)
	}
}

// isPrivateIP checks whether an IP falls within any blocked CIDR range.
func isPrivateIP(ip net.IP) bool {
	for _, ipNet := range privateIPNets {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// safeDialContext returns a DialContext func that rejects connections to private IP ranges.
func safeDialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("split host/port: %w", err)
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve host %q: %w", host, err)
		}
		for _, ip := range ips {
			if isPrivateIP(ip.IP) {
				return nil, errPrivateIP
			}
		}
		// Reconnect to the first allowed IP to prevent TOCTOU with DNS rebinding.
		if len(ips) > 0 {
			addr = net.JoinHostPort(ips[0].IP.String(), port)
		}
		return dialer.DialContext(ctx, network, addr)
	}
}

// reservedHeaders are header names controlled by Raven that custom headers must not override.
var reservedHeaders = map[string]struct{}{
	"content-type":      {},
	"x-raven-signature": {},
	"x-raven-event":     {},
}

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
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		DialContext: safeDialContext(dialer),
	}
	return &WebhookDeliveryHandler{
		pool: pool,
		repo: repo,
		httpClient: &http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
			// Prevent following redirects to internal URLs.
			CheckRedirect: func(req *http.Request, _ []*http.Request) error {
				if !strings.HasPrefix(req.URL.Scheme, "http") {
					return fmt.Errorf("disallowed redirect scheme: %s", req.URL.Scheme)
				}
				return nil
			},
		},
		logger: logger,
	}
}

// ProcessTask implements asynq.Handler for webhook delivery tasks.
func (h *WebhookDeliveryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

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

	// Validate URL scheme to prevent SSRF via non-HTTP protocols.
	if !strings.HasPrefix(hook.URL, "https://") && !strings.HasPrefix(hook.URL, "http://") {
		return fmt.Errorf("%w: webhook URL must use http or https scheme", asynq.SkipRetry)
	}

	// Build and send the HTTP request.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}

	// Set custom headers first so Raven-controlled headers cannot be overridden.
	for k, v := range hook.Headers {
		if _, reserved := reservedHeaders[strings.ToLower(k)]; reserved {
			continue
		}
		req.Header.Set(k, v)
	}
	// Set Raven-controlled headers last to ensure they are not overridden.
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Raven-Signature", signature)
	req.Header.Set("X-Raven-Event", p.EventType)

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

	// Reset failure_count on a successful delivery so subsequent failures start fresh.
	resetErr := db.WithOrgID(ctx, h.pool, p.OrgID, func(tx pgx.Tx) error {
		return h.repo.ResetFailureCount(ctx, tx, p.WebhookID)
	})
	if resetErr != nil {
		h.logger.Warn("webhook: failed to reset failure count",
			slog.String("webhook_id", p.WebhookID),
			slog.String("error", resetErr.Error()),
		)
	}

	h.logger.Info("webhook delivered",
		"webhook_id", p.WebhookID,
		"status", responseStatus,
	)

	return nil
}

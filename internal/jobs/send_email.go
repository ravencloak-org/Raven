package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/repository"
)

// TypeSendEmail is the Asynq task type for sending notification emails.
const TypeSendEmail = "notification:send_email"

// NotificationRepository is the subset of repository.NotificationRepository
// used by the email job handler.
type NotificationRepository interface {
	CreateLog(ctx context.Context, log model.NotificationLog) error
}

// smtpConfig holds SMTP connection settings read from environment variables.
type smtpConfig struct {
	host string
	port string
	user string
	pass string
	from string
}

func loadSMTPConfig() smtpConfig {
	return smtpConfig{
		host: os.Getenv("SMTP_HOST"),
		port: os.Getenv("SMTP_PORT"),
		user: os.Getenv("SMTP_USER"),
		pass: os.Getenv("SMTP_PASS"),
		from: os.Getenv("SMTP_FROM"),
	}
}

// sanitizeHeader strips CR and LF characters from a header value to prevent
// CRLF injection attacks.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

// HandleSendEmail processes outbound email notifications.
//
// It uses SMTP settings from environment variables:
//
//	SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_FROM
//
// Falls back to logging if SMTP_HOST is empty (dev mode).
// Records a delivery log entry (success or failure) per recipient via repo.CreateLog.
func HandleSendEmail(repo NotificationRepository) asynq.HandlerFunc {
	logger := slog.Default()

	return func(ctx context.Context, t *asynq.Task) error {
		var payload model.SendEmailPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			// Payload is corrupt — retrying will never succeed.
			return fmt.Errorf("HandleSendEmail: unmarshal payload: %v: %w", err, asynq.SkipRetry)
		}

		cfg := loadSMTPConfig()
		now := time.Now().UTC()

		devMode := cfg.host == ""

		var successCount, failCount int

		for _, recipient := range payload.Recipients {
			var sendErr error
			if devMode {
				// Dev mode: SMTP is unconfigured — email is NOT sent.
				// Recipient and subject are omitted from logs to avoid storing PII.
				logger.WarnContext(ctx, "HandleSendEmail: SMTP not configured, email not sent (stdout fallback)",
					slog.String("org_id", payload.OrgID),
					slog.String("config_id", payload.ConfigID),
				)
			} else {
				sendErr = sendToOne(cfg, payload, recipient)
				if sendErr != nil {
					// Recipient address is omitted from logs to avoid storing PII.
					logger.WarnContext(ctx, "HandleSendEmail: smtp delivery failed",
						slog.String("org_id", payload.OrgID),
						slog.String("error", sendErr.Error()),
					)
				}
			}

			logEntry := model.NotificationLog{
				OrgID:            payload.OrgID,
				ConfigID:         &payload.ConfigID,
				NotificationType: payload.NotificationType,
				Recipient:        recipient,
				Subject:          payload.Subject,
			}
			switch {
			case sendErr != nil:
				failCount++
				errMsg := sendErr.Error()
				logEntry.Status = model.NotificationStatusFailed
				logEntry.ErrorMessage = &errMsg
			case devMode:
				// Dev-mode fallback: email was not actually sent.
				successCount++
				logEntry.Status = model.NotificationStatusPending
			default:
				successCount++
				logEntry.Status = model.NotificationStatusSent
				logEntry.SentAt = &now
			}
			if logErr := repo.CreateLog(ctx, logEntry); logErr != nil {
				// Recipient address is omitted from logs to avoid storing PII.
				logger.WarnContext(ctx, "HandleSendEmail: failed to write log entry",
					slog.String("org_id", payload.OrgID),
					slog.String("error", logErr.Error()),
				)
			}
		}

		// Only trigger a retry if every recipient failed — partial success is
		// not retried because already-delivered emails would be duplicated.
		if failCount > 0 && successCount == 0 {
			return fmt.Errorf("HandleSendEmail: all %d recipient(s) failed delivery", failCount)
		}
		if failCount > 0 {
			logger.WarnContext(ctx, "HandleSendEmail: partial delivery failure, not retrying",
				slog.String("org_id", payload.OrgID),
				slog.Int("succeeded", successCount),
				slog.Int("failed", failCount),
			)
		}
		return nil
	}
}

// sendToOne delivers an email to a single recipient using net/smtp.
func sendToOne(cfg smtpConfig, payload model.SendEmailPayload, recipient string) error {
	port := cfg.port
	if port == "" {
		port = "587"
	}
	from := cfg.from
	if from == "" {
		from = cfg.user
	}
	addr := cfg.host + ":" + port

	auth := smtp.PlainAuth("", cfg.user, cfg.pass, cfg.host)
	body := buildRawMessage(from, recipient, payload.Subject, payload.Body)

	if err := smtp.SendMail(addr, auth, from, []string{recipient}, []byte(body)); err != nil {
		return fmt.Errorf("smtp.SendMail: %w", err)
	}
	return nil
}

// buildRawMessage assembles a minimal RFC 2822 email message.
// Header values are sanitized to strip CR/LF characters (CRLF injection prevention).
func buildRawMessage(from, to, subject, body string) string {
	from = sanitizeHeader(from)
	to = sanitizeHeader(to)
	subject = sanitizeHeader(subject)
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

// compile-time check that *repository.NotificationRepository satisfies the interface.
var _ NotificationRepository = (*repository.NotificationRepository)(nil)

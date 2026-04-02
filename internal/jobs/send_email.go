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

// HandleSendEmail processes outbound email notifications.
//
// It uses SMTP settings from environment variables:
//
//	SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_FROM
//
// Falls back to logging if SMTP_HOST is empty (dev mode).
// Records a delivery log entry (success or failure) via repo.CreateLog.
func HandleSendEmail(repo NotificationRepository) asynq.HandlerFunc {
	logger := slog.Default()

	return func(ctx context.Context, t *asynq.Task) error {
		var payload model.SendEmailPayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("HandleSendEmail: unmarshal payload: %w", err)
		}

		cfg := loadSMTPConfig()

		var sendErr error
		if cfg.host == "" {
			// Dev mode: log to stdout instead of sending.
			logger.Info("HandleSendEmail: SMTP not configured, logging email",
				slog.String("org_id", payload.OrgID),
				slog.String("config_id", payload.ConfigID),
				slog.String("subject", payload.Subject),
				slog.Any("recipients", payload.Recipients),
			)
		} else {
			sendErr = sendViaSMTP(cfg, payload)
		}

		// Record a log entry per recipient.
		now := time.Now().UTC()
		for _, recipient := range payload.Recipients {
			logEntry := model.NotificationLog{
				OrgID:            payload.OrgID,
				ConfigID:         &payload.ConfigID,
				NotificationType: payload.NotificationType,
				Recipient:        recipient,
				Subject:          payload.Subject,
				Status:           model.NotificationStatusSent,
			}
			if sendErr != nil {
				errMsg := sendErr.Error()
				logEntry.Status = model.NotificationStatusFailed
				logEntry.ErrorMessage = &errMsg
			} else {
				logEntry.SentAt = &now
			}
			if logErr := repo.CreateLog(ctx, logEntry); logErr != nil {
				logger.WarnContext(ctx, "HandleSendEmail: failed to write log entry",
					slog.String("org_id", payload.OrgID),
					slog.String("recipient", recipient),
					slog.String("error", logErr.Error()),
				)
			}
		}

		if sendErr != nil {
			return fmt.Errorf("HandleSendEmail: smtp: %w", sendErr)
		}
		return nil
	}
}

// sendViaSMTP delivers an email to all recipients using net/smtp.
func sendViaSMTP(cfg smtpConfig, payload model.SendEmailPayload) error {
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

	body := buildRawMessage(from, payload.Recipients, payload.Subject, payload.Body)

	if err := smtp.SendMail(addr, auth, from, payload.Recipients, []byte(body)); err != nil {
		return fmt.Errorf("smtp.SendMail: %w", err)
	}
	return nil
}

// buildRawMessage assembles a minimal RFC 2822 email message.
func buildRawMessage(from string, to []string, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

// compile-time check that *repository.NotificationRepository satisfies the interface.
var _ NotificationRepository = (*repository.NotificationRepository)(nil)

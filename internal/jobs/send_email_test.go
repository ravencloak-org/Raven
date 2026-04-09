package jobs

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/model"
)

// mockNotificationRepo records CreateLog calls for assertion.
type mockNotificationRepo struct {
	logs []model.NotificationLog
}

func (m *mockNotificationRepo) CreateLog(_ context.Context, log model.NotificationLog) error {
	m.logs = append(m.logs, log)
	return nil
}

// newSendEmailTask builds a raw *asynq.Task for HandleSendEmail tests.
func newSendEmailTask(t *testing.T, payload model.SendEmailPayload) *asynq.Task {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	return asynq.NewTask("notification:send_email", data)
}

func TestHandleSendEmail_DevMode_LogsPendingStatus(t *testing.T) {
	// With no SMTP_HOST set (dev mode), HandleSendEmail must NOT send email
	// but must record a log entry with status=pending for each recipient.
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USER", "")
	t.Setenv("SMTP_PASS", "")
	t.Setenv("SMTP_FROM", "")

	repo := &mockNotificationRepo{}
	handler := HandleSendEmail(repo)

	payload := model.SendEmailPayload{
		OrgID:            "org-1",
		ConfigID:         "cfg-1",
		NotificationType: model.NotificationTypeAdminDigest,
		Recipients:       []string{"user@example.com", "admin@example.com"},
		Subject:          "Daily digest",
		Body:             "Hello",
	}
	task := newSendEmailTask(t, payload)

	err := handler(context.Background(), task)
	// Dev mode should not error — no SMTP failure.
	require.NoError(t, err)

	// One log entry per recipient.
	require.Len(t, repo.logs, 2)
	for _, log := range repo.logs {
		assert.Equal(t, model.NotificationStatusPending, log.Status,
			"dev-mode email must be logged as pending")
		assert.Equal(t, "org-1", log.OrgID)
		assert.Equal(t, model.NotificationTypeAdminDigest, log.NotificationType)
	}
}

func TestHandleSendEmail_CorruptPayload_ReturnsSkipRetry(t *testing.T) {
	// Corrupt JSON payload must return asynq.SkipRetry so the task is not retried.
	repo := &mockNotificationRepo{}
	handler := HandleSendEmail(repo)

	task := asynq.NewTask("notification:send_email", []byte("NOT JSON"))
	err := handler(context.Background(), task)
	require.Error(t, err)
	// The error must wrap asynq.SkipRetry.
	assert.ErrorIs(t, err, asynq.SkipRetry,
		"corrupt payload must not be retried")
}

func TestHandleSendEmail_NoRecipients_Succeeds(t *testing.T) {
	// Empty recipients list should succeed with no log entries.
	t.Setenv("SMTP_HOST", "")

	repo := &mockNotificationRepo{}
	handler := HandleSendEmail(repo)

	payload := model.SendEmailPayload{
		OrgID:      "org-1",
		ConfigID:   "cfg-1",
		Recipients: []string{},
		Subject:    "Empty",
		Body:       "No one to notify",
	}
	task := newSendEmailTask(t, payload)

	err := handler(context.Background(), task)
	require.NoError(t, err)
	assert.Empty(t, repo.logs, "no log entries for empty recipients")
}

func TestSanitizeHeader_RemovesCRLF(t *testing.T) {
	// sanitizeHeader is unexported; test via buildRawMessage which uses it.
	// We verify that CRLF injection sequences do not appear in the From header.
	msg := buildRawMessage(
		"from\r\njected@evil.com",
		"to@example.com",
		"Legitimate Subject",
		"body",
	)
	// The CR+LF characters should be stripped, so "jected@evil.com" must not
	// appear on its own line (which would indicate header injection succeeded).
	assert.NotContains(t, msg, "\r\njected@evil.com",
		"CR/LF in From header value must be stripped to prevent header injection")
	// The From line must contain only the sanitized value.
	assert.Contains(t, msg, "From: fromjected@evil.com",
		"sanitized From header must keep non-CRLF characters")
}

package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"

	"github.com/ravencloak-org/Raven/internal/model"
)

// TypeEmailSummary is the Asynq task type for a single conversation-summary email.
// Mirrors jobs.TypeEmailSummary — kept here so enqueue-side callers don't have to
// import the jobs package (which pulls in email/AI-worker wiring).
const TypeEmailSummary = "notification:email_summary"

// validateEmailSummaryPayload rejects obviously-malformed inputs at
// enqueue time so producer bugs fail fast instead of blowing up the worker
// with a confusing unmarshal error. An empty SessionID is especially
// dangerous because it collapses every TaskID to the shared literal
// "summary:" which then dedups every future summary email into one task.
func validateEmailSummaryPayload(p model.EmailSummaryPayload) error {
	if p.OrgID == "" {
		return fmt.Errorf("email_summary payload: org_id is required")
	}
	if p.SessionID == "" {
		return fmt.Errorf("email_summary payload: session_id is required")
	}
	if p.UserEmail == "" {
		return fmt.Errorf("email_summary payload: user_email is required")
	}
	if p.UserID == "" {
		return fmt.Errorf("email_summary payload: user_id is required")
	}
	return nil
}

// NewEmailSummaryTask constructs an Asynq task for a conversation summary.
func NewEmailSummaryTask(p model.EmailSummaryPayload) (*asynq.Task, error) {
	if err := validateEmailSummaryPayload(p); err != nil {
		return nil, err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal EmailSummaryPayload: %w", err)
	}
	return asynq.NewTask(TypeEmailSummary, data), nil
}

// EnqueueEmailSummary schedules a single summary email after a short delay.
// The delay gives voice/chat handlers a moment to flush any trailing messages
// into the conversation_sessions row before we read it.
//
// asynq.TaskID is derived from session_id so that repeated enqueues for the
// same session (e.g. user reconnects and the handler fires again) de-duplicate.
func (c *Client) EnqueueEmailSummary(ctx context.Context, p model.EmailSummaryPayload, opts ...asynq.Option) error {
	task, err := NewEmailSummaryTask(p)
	if err != nil {
		return fmt.Errorf("create email_summary task: %w", err)
	}
	baseOpts := []asynq.Option{
		asynq.MaxRetry(c.maxRetry),
		asynq.Queue("default"),
		asynq.TaskID("summary:" + p.SessionID),
	}
	info, err := c.inner.EnqueueContext(ctx, task, append(baseOpts, opts...)...)
	if err != nil {
		return fmt.Errorf("enqueue email_summary task: %w", err)
	}
	c.logger.Info("enqueued task",
		"type", task.Type(),
		"id", info.ID,
		"queue", info.Queue,
		"session_id", p.SessionID,
	)
	return nil
}

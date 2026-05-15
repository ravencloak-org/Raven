package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ResendSender sends Messages via the Resend HTTP API
// (https://resend.com/docs/api-reference/emails/send-email).
type ResendSender struct {
	// APIKey is the Resend secret key (rs_*). Required.
	APIKey string
	// From is the verified sender address, e.g. noreply@ravencloak.org.
	From string

	client *http.Client
	url    string
}

// NewResendSender returns a ResendSender pointed at the production Resend
// API with a 10s request timeout.
func NewResendSender(apiKey, from string) *ResendSender {
	return &ResendSender{
		APIKey: apiKey,
		From:   from,
		client: &http.Client{Timeout: 10 * time.Second},
		url:    "https://api.resend.com/emails",
	}
}

// Send posts the Message to the Resend /emails endpoint. Returns an
// error on transport failures or non-2xx responses.
func (s *ResendSender) Send(ctx context.Context, msg Message) error {
	if s.APIKey == "" {
		return errors.New("resend: API key not configured")
	}
	body, err := json.Marshal(map[string]any{
		"from":    s.From,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"text":    msg.Text,
	})
	if err != nil {
		return fmt.Errorf("resend: marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend: new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: do: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode/100 != 2 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend: non-2xx %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

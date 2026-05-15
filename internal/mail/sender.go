// Package mail defines a provider-agnostic email-sending interface.
package mail

import "context"

// Message is the payload sent by a Sender.
type Message struct {
	To      string
	Subject string
	Text    string
}

// Sender abstracts email transport so the API can swap providers.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// NoopSender records each Message in memory and never errors. Useful for
// tests and for runs without RESEND_API_KEY configured.
type NoopSender struct {
	Sent []Message
}

// Send appends msg to NoopSender.Sent and returns nil.
func (s *NoopSender) Send(_ context.Context, msg Message) error {
	s.Sent = append(s.Sent, msg)
	return nil
}

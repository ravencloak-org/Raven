package mail

import (
	"context"
	"testing"
)

func TestNoopSender_Send_RecordsMessage(t *testing.T) {
	s := &NoopSender{}
	err := s.Send(context.Background(), Message{
		To:      "user@example.com",
		Subject: "Hi",
		Text:    "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(s.Sent))
	}
	if s.Sent[0].To != "user@example.com" {
		t.Fatalf("unexpected To: %s", s.Sent[0].To)
	}
}

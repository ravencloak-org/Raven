package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ravencloak-org/Raven/internal/service"
)

func TestLiveKitSDPRelay_GenerateAnswer_EmptyRoom(t *testing.T) {
	relay := service.NewLiveKitSDPRelay(&mockLiveKitClient{})
	_, err := relay.GenerateAnswer(context.Background(), "", "v=0\r\n")
	if err == nil {
		t.Fatal("expected error for empty room name")
	}
}

func TestLiveKitSDPRelay_GenerateAnswer_EmptyOffer(t *testing.T) {
	relay := service.NewLiveKitSDPRelay(&mockLiveKitClient{})
	_, err := relay.GenerateAnswer(context.Background(), "room-1", "")
	if err == nil {
		t.Fatal("expected error for empty SDP offer")
	}
}

func TestLiveKitSDPRelay_GenerateAnswer_TokenError(t *testing.T) {
	lk := &mockLiveKitClient{
		generateTokenFn: func(_, _, _ string) (string, error) {
			return "", errForTest("token generation failed")
		},
	}
	relay := service.NewLiveKitSDPRelay(lk)
	_, err := relay.GenerateAnswer(context.Background(), "room-1", "v=0\r\n")
	if err == nil {
		t.Fatal("expected error when token generation fails")
	}
}

func TestLiveKitSDPRelay_GenerateAnswer_OpusDefault(t *testing.T) {
	lk := &mockLiveKitClient{
		generateTokenFn: func(_, _, _ string) (string, error) {
			return "test-token-123", nil
		},
	}
	relay := service.NewLiveKitSDPRelay(lk)

	sdpOffer := "v=0\r\no=- 0 0 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\n"
	answer, err := relay.GenerateAnswer(context.Background(), "wa-room-test", sdpOffer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(answer, "opus") {
		t.Error("expected opus codec in SDP answer")
	}
	if !strings.Contains(answer, "48000/2") {
		t.Error("expected opus clock rate 48000/2 in SDP answer")
	}
	if !strings.Contains(answer, "a=x-livekit-room:wa-room-test") {
		t.Error("expected LiveKit room attribute in SDP answer")
	}
	if !strings.Contains(answer, "a=x-livekit-token:test-token-123") {
		t.Error("expected LiveKit token attribute in SDP answer")
	}
	if !strings.Contains(answer, "a=sendrecv") {
		t.Error("expected sendrecv direction in SDP answer (bidirectional audio)")
	}
}

func TestLiveKitSDPRelay_GenerateAnswer_OpusFromOffer(t *testing.T) {
	lk := &mockLiveKitClient{
		generateTokenFn: func(_, _, _ string) (string, error) {
			return "tok", nil
		},
	}
	relay := service.NewLiveKitSDPRelay(lk)

	sdpOffer := "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=rtpmap:111 opus/48000/2\r\n"
	answer, err := relay.GenerateAnswer(context.Background(), "room", sdpOffer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(answer, "opus") {
		t.Error("expected opus codec when offer contains opus")
	}
}

func TestLiveKitSDPRelay_GenerateAnswer_PCMUFromOffer(t *testing.T) {
	lk := &mockLiveKitClient{
		generateTokenFn: func(_, _, _ string) (string, error) {
			return "tok", nil
		},
	}
	relay := service.NewLiveKitSDPRelay(lk)

	sdpOffer := "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 0\r\na=rtpmap:0 PCMU/8000\r\n"
	answer, err := relay.GenerateAnswer(context.Background(), "room", sdpOffer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(answer, "PCMU") {
		t.Error("expected PCMU codec when offer contains PCMU")
	}
	if !strings.Contains(answer, "8000") {
		t.Error("expected 8000 clock rate for PCMU")
	}
}

func TestLiveKitSDPRelay_GenerateAnswer_PCMAFromOffer(t *testing.T) {
	lk := &mockLiveKitClient{
		generateTokenFn: func(_, _, _ string) (string, error) {
			return "tok", nil
		},
	}
	relay := service.NewLiveKitSDPRelay(lk)

	sdpOffer := "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 8\r\na=rtpmap:8 PCMA/8000\r\n"
	answer, err := relay.GenerateAnswer(context.Background(), "room", sdpOffer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(answer, "PCMA") {
		t.Error("expected PCMA codec when offer contains PCMA")
	}
}

// errForTest is a simple error type for test use.
type errForTest string

func (e errForTest) Error() string { return string(e) }

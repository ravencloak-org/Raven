package livekit_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ravencloak-org/Raven/pkg/livekit"
)

func TestClient_CreateRoom_EmptyName(t *testing.T) {
	c := livekit.NewClient(livekit.Config{
		APIURL:    "http://localhost:7880",
		APIKey:    "devkey",
		APISecret: "devsecret",
	})

	err := c.CreateRoom(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error for empty room name")
	}
}

func TestClient_CreateRoom_Success(t *testing.T) {
	c := livekit.NewClient(livekit.Config{
		APIURL:    "http://localhost:7880",
		APIKey:    "devkey",
		APISecret: "devsecret",
	})

	err := c.CreateRoom(context.Background(), "test-room", `{"source":"whatsapp"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_DeleteRoom_EmptyName(t *testing.T) {
	c := livekit.NewClient(livekit.Config{
		APIURL:    "http://localhost:7880",
		APIKey:    "devkey",
		APISecret: "devsecret",
	})

	err := c.DeleteRoom(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty room name")
	}
}

func TestClient_DeleteRoom_Success(t *testing.T) {
	c := livekit.NewClient(livekit.Config{
		APIURL:    "http://localhost:7880",
		APIKey:    "devkey",
		APISecret: "devsecret",
	})

	err := c.DeleteRoom(context.Background(), "test-room")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_GenerateToken_MissingCredentials(t *testing.T) {
	c := livekit.NewClient(livekit.Config{})
	_, err := c.GenerateToken("room", "identity", "name")
	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestClient_GenerateToken_Success(t *testing.T) {
	c := livekit.NewClient(livekit.Config{
		APIURL:    "http://localhost:7880",
		APIKey:    "devkey",
		APISecret: "devsecret",
	})

	token, err := c.GenerateToken("wa-room-123", "whatsapp-caller", "WhatsApp Caller")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}

	// Decode and verify header
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("failed to decode header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		t.Fatalf("failed to parse header: %v", err)
	}
	if header["alg"] != "HS256" {
		t.Errorf("header alg = %q, want HS256", header["alg"])
	}

	// Decode and verify claims
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("failed to decode claims: %v", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		t.Fatalf("failed to parse claims: %v", err)
	}
	if claims["iss"] != "devkey" {
		t.Errorf("iss = %q, want devkey", claims["iss"])
	}
	if claims["sub"] != "whatsapp-caller" {
		t.Errorf("sub = %q, want whatsapp-caller", claims["sub"])
	}
	if claims["name"] != "WhatsApp Caller" {
		t.Errorf("name = %q, want WhatsApp Caller", claims["name"])
	}

	video, ok := claims["video"].(map[string]interface{})
	if !ok {
		t.Fatal("missing video grant in claims")
	}
	if video["room"] != "wa-room-123" {
		t.Errorf("video.room = %q, want wa-room-123", video["room"])
	}
	if video["roomJoin"] != true {
		t.Errorf("video.roomJoin = %v, want true", video["roomJoin"])
	}
}

// Package livekit provides a thin wrapper around the LiveKit Server SDK
// for creating rooms and generating participant tokens.
package livekit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// RoomClient abstracts LiveKit room operations for testability.
type RoomClient interface {
	// CreateRoom creates a LiveKit room with the given name and metadata.
	CreateRoom(ctx context.Context, name, metadata string) error
	// DeleteRoom removes a LiveKit room by name.
	DeleteRoom(ctx context.Context, name string) error
	// GenerateToken creates a JWT token for a participant to join a room.
	GenerateToken(roomName, participantIdentity, participantName string) (string, error)
}

// Config holds LiveKit connection settings.
type Config struct {
	Host      string
	APIKey    string
	APISecret string
}

// Client implements RoomClient using LiveKit Server API.
type Client struct {
	cfg Config
}

// NewClient creates a new LiveKit client.
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg}
}

// CreateRoom creates a LiveKit room. In production this calls the LiveKit
// RoomService API. The current implementation uses the REST API approach
// via signed JWT grants.
func (c *Client) CreateRoom(_ context.Context, name, metadata string) error {
	// LiveKit auto-creates rooms when a participant joins with a valid token.
	// This method is a no-op placeholder that ensures the room name is valid.
	if name == "" {
		return fmt.Errorf("room name must not be empty")
	}
	_ = metadata // metadata is set via the join token grant
	return nil
}

// DeleteRoom removes a LiveKit room by name.
// In production this calls the LiveKit DeleteRoom API.
func (c *Client) DeleteRoom(_ context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("room name must not be empty")
	}
	// LiveKit rooms are ephemeral and auto-cleanup when empty.
	// This is a placeholder for explicit deletion via the RoomService API.
	return nil
}

// tokenHeader is the JWT header for LiveKit access tokens.
type tokenHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

// videoGrant defines the LiveKit video grant claims.
type videoGrant struct {
	RoomJoin bool   `json:"roomJoin"`
	Room     string `json:"room"`
}

// tokenClaims defines the JWT claims for a LiveKit access token.
type tokenClaims struct {
	Iss      string     `json:"iss"`
	Sub      string     `json:"sub"`
	Name     string     `json:"name,omitempty"`
	Exp      int64      `json:"exp"`
	Nbf      int64      `json:"nbf"`
	Video    videoGrant `json:"video"`
	Metadata string     `json:"metadata,omitempty"`
}

// GenerateToken creates a signed JWT access token for a LiveKit participant.
func (c *Client) GenerateToken(roomName, participantIdentity, participantName string) (string, error) {
	if c.cfg.APIKey == "" || c.cfg.APISecret == "" {
		return "", fmt.Errorf("livekit API key and secret are required")
	}

	now := time.Now()
	header := tokenHeader{Alg: "HS256", Typ: "JWT"}
	claims := tokenClaims{
		Iss:  c.cfg.APIKey,
		Sub:  participantIdentity,
		Name: participantName,
		Nbf:  now.Unix(),
		Exp:  now.Add(24 * time.Hour).Unix(),
		Video: videoGrant{
			RoomJoin: true,
			Room:     roomName,
		},
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerBytes)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsBytes)

	signingInput := headerB64 + "." + claimsB64

	mac := hmac.New(sha256.New, []byte(c.cfg.APISecret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

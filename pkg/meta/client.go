// Package meta provides a Go HTTP client for the Meta Graph API (WhatsApp Business Calling).
package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultBaseURL includes the API version; paths are /{phoneNumberID}/calls/...
const defaultBaseURL = "https://graph.facebook.com/v20.0"

// Option is a functional option for configuring a Client.
type Option func(*Client)

// WithBaseURL overrides the Meta Graph API base URL (useful for testing).
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

// CallResponse is the response from the Meta Graph API when initiating a call.
type CallResponse struct {
	CallID    string `json:"call_id"`
	Status    string `json:"status"`
	SDPAnswer string `json:"sdp,omitempty"`
}

// SendSDPAnswerRequest is the payload for sending an SDP answer.
type SendSDPAnswerRequest struct {
	SDPAnswer string `json:"sdp_answer"`
}

// SendSDPAnswerResponse is the response from sending an SDP answer.
type SendSDPAnswerResponse struct {
	Success bool `json:"success"`
}

// CallStatusResponse is the response from getting call status.
type CallStatusResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// sdpPayload is the SDP sub-object in a call initiation request.
type sdpPayload struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

// initiateCallRequest is the request body for POST /{phoneNumberID}/calls.
type initiateCallRequest struct {
	To   string     `json:"to"`
	Type string     `json:"type"`
	SDP  sdpPayload `json:"sdp"`
}

// Client is a thin HTTP client for the Meta Graph API WhatsApp Calling endpoints.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new Meta Graph API client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// readAndClose reads the full response body and closes it, returning errors from either step.
func readAndClose(body io.ReadCloser) ([]byte, error) {
	data, readErr := io.ReadAll(body)
	closeErr := body.Close()
	if readErr != nil {
		return nil, fmt.Errorf("meta: read response: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("meta: close response body: %w", closeErr)
	}
	return data, nil
}

// InitiateCall starts an outbound WhatsApp call to the given number with an SDP offer.
func (c *Client) InitiateCall(ctx context.Context, accessToken, phoneNumberID, to, sdpOffer string) (*CallResponse, error) {
	body := initiateCallRequest{
		To:   to,
		Type: "audio",
		SDP: sdpPayload{
			Type: "offer",
			SDP:  sdpOffer,
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("meta.Client.InitiateCall marshal: %w", err)
	}

	url := fmt.Sprintf("%s/%s/calls", c.baseURL, phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("meta.Client.InitiateCall build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meta.Client.InitiateCall http: %w", err)
	}

	respBody, err := readAndClose(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("meta.Client.InitiateCall: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("meta.Client.InitiateCall non-2xx status %d: %s", resp.StatusCode, string(respBody))
	}

	var callResp CallResponse
	if err := json.Unmarshal(respBody, &callResp); err != nil {
		return nil, fmt.Errorf("meta.Client.InitiateCall unmarshal: %w", err)
	}
	return &callResp, nil
}

// SendSDPAnswer sends an SDP answer to an existing call identified by callID.
func (c *Client) SendSDPAnswer(ctx context.Context, accessToken, phoneNumberID, callID, sdpAnswer string) (*SendSDPAnswerResponse, error) {
	body := SendSDPAnswerRequest{SDPAnswer: sdpAnswer}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("meta.Client.SendSDPAnswer marshal: %w", err)
	}

	url := fmt.Sprintf("%s/%s/calls/%s", c.baseURL, phoneNumberID, callID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("meta.Client.SendSDPAnswer build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meta.Client.SendSDPAnswer http: %w", err)
	}

	respBody, err := readAndClose(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("meta.Client.SendSDPAnswer: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("meta.Client.SendSDPAnswer non-2xx status %d: %s", resp.StatusCode, string(respBody))
	}

	var result SendSDPAnswerResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("meta.Client.SendSDPAnswer unmarshal: %w", err)
	}
	return &result, nil
}

// GetCallStatus retrieves the status of an active call.
func (c *Client) GetCallStatus(ctx context.Context, accessToken, phoneNumberID, callID string) (*CallStatusResponse, error) {
	url := fmt.Sprintf("%s/%s/calls/%s", c.baseURL, phoneNumberID, callID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("meta.Client.GetCallStatus build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("meta.Client.GetCallStatus http: %w", err)
	}

	respBody, err := readAndClose(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("meta.Client.GetCallStatus: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("meta.Client.GetCallStatus non-2xx status %d: %s", resp.StatusCode, string(respBody))
	}

	var result CallStatusResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("meta.Client.GetCallStatus unmarshal: %w", err)
	}
	return &result, nil
}

// EndCall terminates an active WhatsApp call.
func (c *Client) EndCall(ctx context.Context, accessToken, phoneNumberID, callID string) error {
	url := fmt.Sprintf("%s/%s/calls/%s", c.baseURL, phoneNumberID, callID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("meta.Client.EndCall build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("meta.Client.EndCall http: %w", err)
	}

	respBody, err := readAndClose(resp.Body)
	if err != nil {
		return fmt.Errorf("meta.Client.EndCall: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("meta.Client.EndCall non-2xx status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

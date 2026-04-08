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

const (
	defaultAPIVersion = "v20.0"
	defaultBaseURL    = "https://graph.facebook.com"
)

// CallResponse is the response from the Meta Graph API when initiating a call.
type CallResponse struct {
	CallID    string `json:"call_id"`
	Status    string `json:"status"`
	SDPAnswer string `json:"sdp,omitempty"`
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

// sendSDPAnswerRequest is the request body for sending an SDP answer.
type sendSDPAnswerRequest struct {
	SDP sdpPayload `json:"sdp"`
}

// Client is a thin HTTP client for the Meta Graph API WhatsApp Calling endpoints.
type Client struct {
	httpClient    *http.Client
	accessToken   string
	phoneNumberID string
	apiVersion    string
	baseURL       string
}

// NewClient creates a new Meta Graph API client for the given phone number.
func NewClient(accessToken, phoneNumberID string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		accessToken:   accessToken,
		phoneNumberID: phoneNumberID,
		apiVersion:    defaultAPIVersion,
		baseURL:       defaultBaseURL,
	}
}

// InitiateCall starts an outbound WhatsApp call to the given number with an SDP offer.
// It posts to POST /{apiVersion}/{phoneNumberID}/calls with the SDP offer embedded.
func (c *Client) InitiateCall(ctx context.Context, to string, sdpOffer string) (*CallResponse, error) {
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

	url := fmt.Sprintf("%s/%s/%s/calls", c.baseURL, c.apiVersion, c.phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("meta.Client.InitiateCall build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

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
func (c *Client) SendSDPAnswer(ctx context.Context, callID string, sdpAnswer string) error {
	body := sendSDPAnswerRequest{
		SDP: sdpPayload{
			Type: "answer",
			SDP:  sdpAnswer,
		},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("meta.Client.SendSDPAnswer marshal: %w", err)
	}

	url := fmt.Sprintf("%s/%s/%s/calls/%s", c.baseURL, c.apiVersion, c.phoneNumberID, callID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("meta.Client.SendSDPAnswer build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("meta.Client.SendSDPAnswer http: %w", err)
	}

	respBody, err := readAndClose(resp.Body)
	if err != nil {
		return fmt.Errorf("meta.Client.SendSDPAnswer: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("meta.Client.SendSDPAnswer non-2xx status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// EndCall terminates an active WhatsApp call.
func (c *Client) EndCall(ctx context.Context, callID string) error {
	url := fmt.Sprintf("%s/%s/%s/calls/%s", c.baseURL, c.apiVersion, c.phoneNumberID, callID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("meta.Client.EndCall build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

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

// Package hyperswitch provides an HTTP client for the Hyperswitch payment
// orchestration API. Hyperswitch acts as the routing layer and connects to
// downstream payment processors (e.g. Razorpay for UPI/card in India).
package hyperswitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client communicates with the Hyperswitch API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewClient creates a Client for the given Hyperswitch instance.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
	}
}

// CreatePaymentRequest is the request body for POST /payments.
type CreatePaymentRequest struct {
	Amount           int64             `json:"amount"`
	Currency         string            `json:"currency"`
	CustomerID       string            `json:"customer_id,omitempty"`
	SetupFutureUsage string            `json:"setup_future_usage,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// PaymentResponse is the relevant subset of the Hyperswitch /payments response.
type PaymentResponse struct {
	PaymentID    string `json:"payment_id"`
	ClientSecret string `json:"client_secret"`
	Status       string `json:"status"`
}

// CreatePayment calls POST /payments to create a payment intent.
func (c *Client) CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*PaymentResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("hyperswitch: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/payments", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("hyperswitch: create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("hyperswitch: execute request: %w", err)
	}
	defer resp.Body.Close()

	return decodeResponse[PaymentResponse](resp)
}

// CancelPayment calls POST /payments/{id}/cancel.
func (c *Client) CancelPayment(ctx context.Context, paymentID string) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/payments/"+paymentID+"/cancel", nil)
	if err != nil {
		return fmt.Errorf("hyperswitch: create cancel request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("hyperswitch: execute cancel request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respData, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hyperswitch: cancel failed (status %d): %s", resp.StatusCode, string(respData))
	}
	return nil
}

// GetPayment calls GET /payments/{id} to retrieve payment status.
func (c *Client) GetPayment(ctx context.Context, paymentID string) (*PaymentResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/payments/"+paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("hyperswitch: create get request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("hyperswitch: execute get request: %w", err)
	}
	defer resp.Body.Close()

	return decodeResponse[PaymentResponse](resp)
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("api-key", c.apiKey)
}

func decodeResponse[T any](resp *http.Response) (*T, error) {
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("hyperswitch: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hyperswitch: API error (status %d): %s", resp.StatusCode, string(respData))
	}

	var result T
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("hyperswitch: decode response: %w", err)
	}
	return &result, nil
}

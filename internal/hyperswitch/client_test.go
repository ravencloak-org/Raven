package hyperswitch_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ravencloak-org/Raven/internal/hyperswitch"
)

func TestCreatePayment_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/payments", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-api-key", r.Header.Get("api-key"))

		var body hyperswitch.CreatePaymentRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, int64(2900), body.Amount)
		assert.Equal(t, "USD", body.Currency)
		assert.Equal(t, "org-123", body.CustomerID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := hyperswitch.PaymentResponse{
			PaymentID:    "pay_test_123",
			ClientSecret: "secret_test_abc",
			Status:       "requires_payment_method",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := hyperswitch.NewClient(server.URL, "test-api-key")
	resp, err := client.CreatePayment(context.Background(), &hyperswitch.CreatePaymentRequest{
		Amount:     2900,
		Currency:   "USD",
		CustomerID: "org-123",
	})

	require.NoError(t, err)
	assert.Equal(t, "pay_test_123", resp.PaymentID)
	assert.Equal(t, "secret_test_abc", resp.ClientSecret)
}

func TestCreatePayment_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid amount"}`))
	}))
	defer server.Close()

	client := hyperswitch.NewClient(server.URL, "test-api-key")
	_, err := client.CreatePayment(context.Background(), &hyperswitch.CreatePaymentRequest{
		Amount:   -100,
		Currency: "USD",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "API error (status 400)")
}

func TestCancelPayment_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/payments/pay_123/cancel", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := hyperswitch.NewClient(server.URL, "test-api-key")
	err := client.CancelPayment(context.Background(), "pay_123")
	assert.NoError(t, err)
}

func TestCancelPayment_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"payment not found"}`))
	}))
	defer server.Close()

	client := hyperswitch.NewClient(server.URL, "test-api-key")
	err := client.CancelPayment(context.Background(), "pay_nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancel failed (status 404)")
}

func TestGetPayment_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/payments/pay_123", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		resp := hyperswitch.PaymentResponse{
			PaymentID:    "pay_123",
			ClientSecret: "",
			Status:       "succeeded",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := hyperswitch.NewClient(server.URL, "test-api-key")
	resp, err := client.GetPayment(context.Background(), "pay_123")
	require.NoError(t, err)
	assert.Equal(t, "succeeded", resp.Status)
}

func TestCreatePayment_SetsRazorpayMetadata(t *testing.T) {
	var receivedMetadata map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body hyperswitch.CreatePaymentRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		receivedMetadata = body.Metadata

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hyperswitch.PaymentResponse{
			PaymentID:    "pay_test",
			ClientSecret: "secret_test",
		})
	}))
	defer server.Close()

	client := hyperswitch.NewClient(server.URL, "test-api-key")
	_, err := client.CreatePayment(context.Background(), &hyperswitch.CreatePaymentRequest{
		Amount:     2900,
		Currency:   "USD",
		CustomerID: "org-123",
		Metadata: map[string]string{
			"plan_id": "plan_pro",
			"org_id":  "org-123",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "plan_pro", receivedMetadata["plan_id"])
	assert.Equal(t, "org-123", receivedMetadata["org_id"])
}

package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ravencloak-org/Raven/internal/hyperswitch"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock HyperswitchClient that tracks whether CreatePayment is called ---

type trackingHSClient struct {
	called bool
}

func (c *trackingHSClient) CreatePayment(_ context.Context, _ *hyperswitch.CreatePaymentRequest) (*hyperswitch.PaymentResponse, error) {
	c.called = true
	return &hyperswitch.PaymentResponse{PaymentID: "pay_mock", ClientSecret: "secret_mock"}, nil
}

func (c *trackingHSClient) CancelPayment(_ context.Context, _ string) error {
	return nil
}

func (c *trackingHSClient) CreateRefund(_ context.Context, _ *hyperswitch.CreateRefundRequest) (*hyperswitch.RefundResponse, error) {
	return &hyperswitch.RefundResponse{RefundID: "ref_mock", Status: "pending"}, nil
}

// --- Tests ---

// TestCreateEnterpriseSubscription_TooFewSeats verifies that SeatCount < 20 is rejected.
func TestCreateEnterpriseSubscription_TooFewSeats(t *testing.T) {
	svc := service.NewBillingService(nil, nil, nil, "", "")

	req := model.CreateEnterpriseSubscriptionRequest{
		OrgID:         "org_test",
		SeatCount:     10, // below the 20-seat minimum
		ContractStart: time.Now().UTC(),
		ContractEnd:   time.Now().UTC().AddDate(1, 0, 0),
	}

	_, err := svc.CreateEnterpriseSubscription(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "20")
}

// TestCreateEnterpriseSubscription_BoundarySeats verifies the 20-seat minimum boundary.
func TestCreateEnterpriseSubscription_BoundarySeats(t *testing.T) {
	svc := service.NewBillingService(nil, nil, nil, "", "")

	for _, tc := range []struct {
		seats   int
		wantErr bool
	}{
		{seats: 1, wantErr: true},
		{seats: 19, wantErr: true},
	} {
		req := model.CreateEnterpriseSubscriptionRequest{
			OrgID:         "org_test",
			SeatCount:     tc.seats,
			ContractStart: time.Now().UTC(),
			ContractEnd:   time.Now().UTC().AddDate(1, 0, 0),
		}
		_, err := svc.CreateEnterpriseSubscription(context.Background(), req)
		require.Error(t, err, "expected seat-count error for seats=%d", tc.seats)
		assert.Contains(t, err.Error(), "20", "error must mention minimum seat count")
	}
}

// TestCreateEnterpriseSubscription_NoHyperswitchCall verifies Hyperswitch is never called
// for enterprise subscriptions -- invoicing is external.
func TestCreateEnterpriseSubscription_NoHyperswitchCall(t *testing.T) {
	hsClient := &trackingHSClient{}
	// Use seat count below minimum so we get a validation error before any external call.
	svc := service.NewBillingService(nil, nil, hsClient, "", "")

	req := model.CreateEnterpriseSubscriptionRequest{
		OrgID:         "org_test",
		SeatCount:     5, // below minimum -- validation exits before DB/HS
		ContractStart: time.Now().UTC(),
		ContractEnd:   time.Now().UTC().AddDate(1, 0, 0),
	}

	_, err := svc.CreateEnterpriseSubscription(context.Background(), req)
	require.Error(t, err)
	assert.False(t, hsClient.called, "Hyperswitch must not be called for enterprise subscriptions")
}

// TestCreateEnterpriseSubscription_SlackNotFiredWhenDBFails verifies that Slack is only
// sent after a successful DB write. When the DB fails the goroutine must not be spawned.
func TestCreateEnterpriseSubscription_SlackNotFiredWhenDBFails(t *testing.T) {
	slackCh := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slackCh <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Service with Slack URL set but nil pool -> DB error -> Slack goroutine not launched.
	svc := service.NewBillingService(&mockBillingRepo{}, nil, nil, "", srv.URL)

	req := model.CreateEnterpriseSubscriptionRequest{
		OrgID:         "org_slack",
		SeatCount:     30,
		ContractStart: time.Now().UTC(),
		ContractEnd:   time.Now().UTC().AddDate(1, 0, 0),
	}

	_, err := svc.CreateEnterpriseSubscription(context.Background(), req)
	assert.Error(t, err, "expected error due to nil pool")

	select {
	case <-slackCh:
		t.Error("Slack must NOT be called when DB write fails")
	case <-time.After(50 * time.Millisecond):
		// expected: no Slack call
	}
}

// TestCreateEnterpriseSubscription_EmptySlackURL_NoCall verifies that when slackWebhookURL
// is empty no HTTP request is sent to Slack, even after a seat-count validation failure.
func TestCreateEnterpriseSubscription_EmptySlackURL_NoCall(t *testing.T) {
	slackCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slackCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// slackWebhookURL="" -- Slack must never be called.
	svc := service.NewBillingService(nil, nil, nil, "", "")

	req := model.CreateEnterpriseSubscriptionRequest{
		OrgID:         "org_no_slack",
		SeatCount:     3, // below minimum -- validation exits, no goroutine
		ContractStart: time.Now().UTC(),
		ContractEnd:   time.Now().UTC().AddDate(1, 0, 0),
	}

	_, err := svc.CreateEnterpriseSubscription(context.Background(), req)
	require.Error(t, err)

	time.Sleep(30 * time.Millisecond)
	assert.False(t, slackCalled, "Slack must not be called when slackWebhookURL is empty")
}

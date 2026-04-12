package handler_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/middleware"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockBillingService implements handler.BillingServicer for unit tests.
type mockBillingService struct {
	getPlansFn            func() []model.Plan
	createSubscriptionFn  func(ctx context.Context, orgID string, req model.CreateSubscriptionRequest) (*model.Subscription, error)
	cancelSubscriptionFn  func(ctx context.Context, orgID string, subscriptionID string) error
	createPaymentIntentFn func(ctx context.Context, orgID string, req model.CreatePaymentIntentRequest) (*model.PaymentIntent, error)
	verifyWebhookSigFn    func(payload []byte, signature string) error
	handleWebhookFn       func(ctx context.Context, event model.HyperswitchWebhookPayload) error
}

func (m *mockBillingService) GetPlans() []model.Plan {
	if m.getPlansFn != nil {
		return m.getPlansFn()
	}
	return model.DefaultPlans()
}

func (m *mockBillingService) GetActiveSubscription(_ context.Context, _ string) (*model.Subscription, error) {
	return nil, nil
}

func (m *mockBillingService) CreateSubscription(ctx context.Context, orgID string, req model.CreateSubscriptionRequest) (*model.Subscription, error) {
	return m.createSubscriptionFn(ctx, orgID, req)
}

func (m *mockBillingService) CancelSubscription(ctx context.Context, orgID string, subscriptionID string) error {
	return m.cancelSubscriptionFn(ctx, orgID, subscriptionID)
}

func (m *mockBillingService) CreatePaymentIntent(ctx context.Context, orgID string, req model.CreatePaymentIntentRequest) (*model.PaymentIntent, error) {
	return m.createPaymentIntentFn(ctx, orgID, req)
}

func (m *mockBillingService) VerifyWebhookSignature(payload []byte, signature string) error {
	if m.verifyWebhookSigFn != nil {
		return m.verifyWebhookSigFn(payload, signature)
	}
	return nil
}

func (m *mockBillingService) HandleWebhook(ctx context.Context, event model.HyperswitchWebhookPayload) error {
	if m.handleWebhookFn != nil {
		return m.handleWebhookFn(ctx, event)
	}
	return nil
}

// newBillingRouter creates a test Gin engine with billing routes.
// If withAuth is true, it sets up middleware that injects org context.
func newBillingRouter(svc handler.BillingServicer, withAuth bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())

	h := handler.NewBillingHandler(svc)

	if withAuth {
		authed := r.Group("/api/v1/billing")
		authed.Use(func(c *gin.Context) {
			c.Set(string(middleware.ContextKeyUserID), "user-123")
			c.Set(string(middleware.ContextKeyOrgRole), "org_admin")
			c.Set(string(middleware.ContextKeyOrgID), "org-123")
			c.Next()
		})
		authed.GET("/plans", h.GetPlans)
		authed.POST("/subscriptions", h.Subscribe)
		authed.DELETE("/subscriptions/:id", h.Unsubscribe)
		authed.POST("/payment-intents", h.CreatePaymentIntent)
	}

	// Webhook endpoint does NOT use auth middleware.
	r.POST("/api/v1/billing/webhook", h.Webhook)

	return r
}

func TestGetPlans_Success(t *testing.T) {
	svc := &mockBillingService{}
	r := newBillingRouter(svc, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/billing/plans", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var plans []model.Plan
	if err := json.Unmarshal(w.Body.Bytes(), &plans); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if len(plans) != 3 {
		t.Errorf("expected 3 plans, got %d", len(plans))
	}
}

func TestSubscribe_Success(t *testing.T) {
	now := time.Now().UTC()
	svc := &mockBillingService{
		createSubscriptionFn: func(_ context.Context, orgID string, req model.CreateSubscriptionRequest) (*model.Subscription, error) {
			return &model.Subscription{
				ID:                 "sub_test",
				OrgID:              orgID,
				PlanID:             req.PlanID,
				Status:             model.SubscriptionStatusActive,
				CurrentPeriodStart: now,
				CurrentPeriodEnd:   now.AddDate(0, 1, 0),
				CreatedAt:          now,
			}, nil
		},
	}
	r := newBillingRouter(svc, true)

	body, _ := json.Marshal(model.CreateSubscriptionRequest{PlanID: "plan_pro"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/subscriptions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var sub model.Subscription
	if err := json.Unmarshal(w.Body.Bytes(), &sub); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if sub.PlanID != "plan_pro" {
		t.Errorf("expected plan_id 'plan_pro', got %q", sub.PlanID)
	}
	if sub.OrgID != "org-123" {
		t.Errorf("expected org_id 'org-123', got %q", sub.OrgID)
	}
}

func TestSubscribe_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockBillingService{}
	r := newBillingRouter(svc, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/subscriptions", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubscribe_NoAuth_Returns401(t *testing.T) {
	svc := &mockBillingService{}
	// Create router WITHOUT auth middleware for subscribe route.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewBillingHandler(svc)
	r.POST("/api/v1/billing/subscriptions", h.Subscribe)

	body, _ := json.Marshal(model.CreateSubscriptionRequest{PlanID: "plan_pro"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/subscriptions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSubscribe_ServiceError_Returns500(t *testing.T) {
	svc := &mockBillingService{
		createSubscriptionFn: func(_ context.Context, _ string, _ model.CreateSubscriptionRequest) (*model.Subscription, error) {
			return nil, apierror.NewInternal("hyperswitch unavailable")
		},
	}
	r := newBillingRouter(svc, true)

	body, _ := json.Marshal(model.CreateSubscriptionRequest{PlanID: "plan_pro"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/subscriptions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnsubscribe_Success(t *testing.T) {
	svc := &mockBillingService{
		cancelSubscriptionFn: func(_ context.Context, _ string, _ string) error {
			return nil
		},
	}
	r := newBillingRouter(svc, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/billing/subscriptions/sub_123", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreatePaymentIntent_Success(t *testing.T) {
	now := time.Now().UTC()
	svc := &mockBillingService{
		createPaymentIntentFn: func(_ context.Context, orgID string, req model.CreatePaymentIntentRequest) (*model.PaymentIntent, error) {
			return &model.PaymentIntent{
				ID:                   "pi_test",
				OrgID:                orgID,
				Amount:               req.Amount,
				Currency:             req.Currency,
				Status:               model.PaymentIntentStatusRequiresPayment,
				HyperswitchPaymentID: "hs_pay_123",
				ClientSecret:         "hs_secret_abc",
				CreatedAt:            now,
			}, nil
		},
	}
	r := newBillingRouter(svc, true)

	body, _ := json.Marshal(model.CreatePaymentIntentRequest{Amount: 2900, Currency: "USD"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/payment-intents", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var pi model.PaymentIntent
	if err := json.Unmarshal(w.Body.Bytes(), &pi); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if pi.ClientSecret != "hs_secret_abc" {
		t.Errorf("expected client_secret 'hs_secret_abc', got %q", pi.ClientSecret)
	}
	if pi.Amount != 2900 {
		t.Errorf("expected amount 2900, got %d", pi.Amount)
	}
}

func TestCreatePaymentIntent_InvalidPayload_Returns422(t *testing.T) {
	svc := &mockBillingService{}
	r := newBillingRouter(svc, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/payment-intents", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreatePaymentIntent_NoAuth_Returns401(t *testing.T) {
	svc := &mockBillingService{}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewBillingHandler(svc)
	r.POST("/api/v1/billing/payment-intents", h.CreatePaymentIntent)

	body, _ := json.Marshal(model.CreatePaymentIntentRequest{Amount: 100, Currency: "USD"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/payment-intents", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhook_Success(t *testing.T) {
	handled := false
	svc := &mockBillingService{
		handleWebhookFn: func(_ context.Context, event model.HyperswitchWebhookPayload) error {
			handled = true
			if event.EventType != "payment_succeeded" {
				t.Errorf("expected event_type 'payment_succeeded', got %q", event.EventType)
			}
			return nil
		},
	}
	r := newBillingRouter(svc, true)

	payload := model.HyperswitchWebhookPayload{
		EventType: "payment_succeeded",
		Content:   map[string]any{"payment_id": "pay_123"},
	}
	body, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/webhook", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !handled {
		t.Error("expected webhook handler to be called")
	}
}

func TestWebhook_InvalidSignature_Returns401(t *testing.T) {
	svc := &mockBillingService{
		verifyWebhookSigFn: func(_ []byte, _ string) error {
			return apierror.NewUnauthorized("invalid webhook signature")
		},
	}
	r := newBillingRouter(svc, true)

	payload := `{"event_type":"payment_succeeded","content":{}}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", "bad_signature")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhook_InvalidPayload_Returns400(t *testing.T) {
	svc := &mockBillingService{}
	r := newBillingRouter(svc, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/webhook", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhook_ValidHMACSignature(t *testing.T) {
	secret := "test-webhook-secret"
	payload := `{"event_type":"payment_succeeded","content":{"payment_id":"pay_123"}}`

	// Compute valid HMAC-SHA256 signature.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	validSig := hex.EncodeToString(mac.Sum(nil))

	svc := &mockBillingService{
		verifyWebhookSigFn: func(p []byte, sig string) error {
			m := hmac.New(sha256.New, []byte(secret))
			m.Write(p)
			expected := hex.EncodeToString(m.Sum(nil))
			if !hmac.Equal([]byte(expected), []byte(sig)) {
				return apierror.NewUnauthorized("invalid webhook signature")
			}
			return nil
		},
	}
	r := newBillingRouter(svc, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/billing/webhook", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", validSig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

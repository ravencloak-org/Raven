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

	"github.com/gin-gonic/gin"

	"github.com/ravencloak-org/Raven/internal/handler"
	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockWhatsAppCallService implements handler.WhatsAppCallServicer for unit tests.
type mockWhatsAppCallService struct {
	handleCallStartedFn   func(ctx context.Context, phoneNumberID, callID, from, to, sdpOffer string) (*model.WhatsAppCall, error)
	handleCallConnectedFn func(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error)
	handleCallEndedFn     func(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error)
}

func (m *mockWhatsAppCallService) HandleCallStarted(ctx context.Context, phoneNumberID, callID, from, to, sdpOffer string) (*model.WhatsAppCall, error) {
	if m.handleCallStartedFn != nil {
		return m.handleCallStartedFn(ctx, phoneNumberID, callID, from, to, sdpOffer)
	}
	return &model.WhatsAppCall{}, nil
}

func (m *mockWhatsAppCallService) HandleCallConnected(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error) {
	if m.handleCallConnectedFn != nil {
		return m.handleCallConnectedFn(ctx, phoneNumberID, callID)
	}
	return &model.WhatsAppCall{}, nil
}

func (m *mockWhatsAppCallService) HandleCallEnded(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error) {
	if m.handleCallEndedFn != nil {
		return m.handleCallEndedFn(ctx, phoneNumberID, callID)
	}
	return &model.WhatsAppCall{}, nil
}

// newMetaWebhookRouter builds a test router for the Meta webhook handler.
func newMetaWebhookRouter(appSecret, verifyToken string, svc handler.WhatsAppCallServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewMetaWebhookHandler(appSecret, verifyToken, svc)
	r.GET("/webhooks/meta", h.VerifyWebhook)
	r.POST("/webhooks/meta", h.HandleEvent)
	return r
}

// makeSignature computes the sha256= HMAC signature for a payload and secret.
func makeSignature(secret, payload string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// --- VerifyWebhook Tests ---

func TestMetaVerifyWebhook_Success(t *testing.T) {
	r := newMetaWebhookRouter("secret", "my-verify-token", nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet,
		"/webhooks/meta?hub.mode=subscribe&hub.verify_token=my-verify-token&hub.challenge=challenge-abc",
		nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "challenge-abc" {
		t.Errorf("body = %q, want 'challenge-abc'", w.Body.String())
	}
}

func TestMetaVerifyWebhook_BadMode_Returns400(t *testing.T) {
	r := newMetaWebhookRouter("secret", "token", nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet,
		"/webhooks/meta?hub.mode=unsubscribe&hub.verify_token=token&hub.challenge=c",
		nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMetaVerifyWebhook_WrongToken_Returns401(t *testing.T) {
	r := newMetaWebhookRouter("secret", "correct-token", nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet,
		"/webhooks/meta?hub.mode=subscribe&hub.verify_token=wrong-token&hub.challenge=c",
		nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMetaVerifyWebhook_MissingChallenge_Returns400(t *testing.T) {
	r := newMetaWebhookRouter("secret", "token", nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet,
		"/webhooks/meta?hub.mode=subscribe&hub.verify_token=token",
		nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- HMAC Verification Tests ---

func TestHMAC_ValidSignature(t *testing.T) {
	secret := "test-app-secret"
	payload := `{"object":"whatsapp_business_account","entry":[]}`
	sig := makeSignature(secret, payload)

	r := newMetaWebhookRouter(secret, "token", nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHMAC_InvalidSignature_Returns401(t *testing.T) {
	secret := "test-app-secret"
	payload := `{"object":"whatsapp_business_account","entry":[]}`

	r := newMetaWebhookRouter(secret, "token", nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=badbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadbadb")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHMAC_MissingSignaturePrefix_Returns401(t *testing.T) {
	secret := "test-app-secret"
	payload := `{"object":"whatsapp_business_account","entry":[]}`

	r := newMetaWebhookRouter(secret, "token", nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	// No "sha256=" prefix.
	req.Header.Set("X-Hub-Signature-256", "justahexstring")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHMAC_EmptySecret_SkipsVerification(t *testing.T) {
	// When appSecret is empty, HMAC verification is skipped (dev mode).
	payload := `{"object":"whatsapp_business_account","entry":[]}`

	r := newMetaWebhookRouter("", "token", nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	r.ServeHTTP(w, req)

	// Should pass even with a bad signature because secret is empty.
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (no secret), got %d: %s", w.Code, w.Body.String())
	}
}

// --- Event Routing Tests ---

func metaPayload(status, callID, phoneNumberID, from, to, sdp string) []byte {
	p := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{
			{
				"id": "WABA_ID",
				"changes": []map[string]any{
					{
						"field": "calls",
						"value": map[string]any{
							"messaging_product": "whatsapp",
							"metadata": map[string]any{
								"display_phone_number": "+1234567890",
								"phone_number_id":      phoneNumberID,
							},
							"calls": []map[string]any{
								{
									"id":        callID,
									"from":      from,
									"to":        to,
									"status":    status,
									"sdp":       sdp,
									"timestamp": "1712345678",
								},
							},
						},
					},
				},
			},
		},
	}
	b, _ := json.Marshal(p)
	return b
}

func TestMetaHandleEvent_Ringing_RoutesToCallStarted(t *testing.T) {
	called := false
	svc := &mockWhatsAppCallService{
		handleCallStartedFn: func(_ context.Context, phoneNumberID, callID, from, to, sdpOffer string) (*model.WhatsAppCall, error) {
			called = true
			if phoneNumberID != "phone-id-1" {
				t.Errorf("phoneNumberID = %q, want 'phone-id-1'", phoneNumberID)
			}
			if callID != "call-ring-1" {
				t.Errorf("callID = %q, want 'call-ring-1'", callID)
			}
			if from != "+15551112222" {
				t.Errorf("from = %q, want '+15551112222'", from)
			}
			if sdpOffer != "v=0 sdp" {
				t.Errorf("sdpOffer = %q, want 'v=0 sdp'", sdpOffer)
			}
			return &model.WhatsAppCall{CallID: callID, State: model.WhatsAppCallStateRinging}, nil
		},
	}

	body := metaPayload("ringing", "call-ring-1", "phone-id-1", "+15551112222", "+15553334444", "v=0 sdp")
	sig := makeSignature("secret", string(body))

	r := newMetaWebhookRouter("secret", "token", svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("HandleCallStarted was not called")
	}
}

func TestMetaHandleEvent_Answered_RoutesToCallConnected(t *testing.T) {
	called := false
	svc := &mockWhatsAppCallService{
		handleCallConnectedFn: func(_ context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error) {
			called = true
			if callID != "call-ans-1" {
				t.Errorf("callID = %q, want 'call-ans-1'", callID)
			}
			return &model.WhatsAppCall{CallID: callID, State: model.WhatsAppCallStateConnected}, nil
		},
	}

	body := metaPayload("answered", "call-ans-1", "phone-id-2", "+1", "+2", "")
	sig := makeSignature("secret", string(body))

	r := newMetaWebhookRouter("secret", "token", svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("HandleCallConnected was not called")
	}
}

func TestMetaHandleEvent_Ended_RoutesToCallEnded(t *testing.T) {
	called := false
	svc := &mockWhatsAppCallService{
		handleCallEndedFn: func(_ context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error) {
			called = true
			if callID != "call-end-1" {
				t.Errorf("callID = %q, want 'call-end-1'", callID)
			}
			return &model.WhatsAppCall{CallID: callID, State: model.WhatsAppCallStateEnded}, nil
		},
	}

	body := metaPayload("ended", "call-end-1", "phone-id-3", "+1", "+2", "")
	sig := makeSignature("secret", string(body))

	r := newMetaWebhookRouter("secret", "token", svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !called {
		t.Error("HandleCallEnded was not called")
	}
}

func TestMetaHandleEvent_UnknownStatus_StillReturns200(t *testing.T) {
	// Unknown statuses should be logged and skipped, not cause a 4xx/5xx.
	svc := &mockWhatsAppCallService{}

	body := metaPayload("holding", "call-hold-1", "phone-id-4", "+1", "+2", "")
	sig := makeSignature("secret", string(body))

	r := newMetaWebhookRouter("secret", "token", svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for unknown status, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMetaHandleEvent_NilCallService_StillReturns200(t *testing.T) {
	// When callSvc is nil, events should be logged but no panic and 200 returned.
	body := metaPayload("ringing", "call-nil-1", "phone-id-5", "+1", "+2", "v=0")
	sig := makeSignature("secret", string(body))

	r := newMetaWebhookRouter("secret", "token", nil) // nil service
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with nil svc, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMetaHandleEvent_InvalidPayload_Returns400(t *testing.T) {
	body := "not valid json"
	sig := makeSignature("secret", body)

	r := newMetaWebhookRouter("secret", "token", nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMetaHandleEvent_MultipleEntries(t *testing.T) {
	// Ensure all entries and changes are processed.
	startedCount := 0
	svc := &mockWhatsAppCallService{
		handleCallStartedFn: func(_ context.Context, _, _, _, _, _ string) (*model.WhatsAppCall, error) {
			startedCount++
			return &model.WhatsAppCall{}, nil
		},
	}

	p := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{
			{
				"id": "WABA_1",
				"changes": []map[string]any{
					{
						"field": "calls",
						"value": map[string]any{
							"metadata": map[string]any{"phone_number_id": "ph1"},
							"calls": []map[string]any{
								{"id": "c1", "from": "+1", "to": "+2", "status": "ringing", "timestamp": "1"},
								{"id": "c2", "from": "+3", "to": "+4", "status": "ringing", "timestamp": "2"},
							},
						},
					},
				},
			},
		},
	}
	body, _ := json.Marshal(p)
	sig := makeSignature("secret", string(body))

	r := newMetaWebhookRouter("secret", "token", svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/meta", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", sig)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if startedCount != 2 {
		t.Errorf("HandleCallStarted called %d times, want 2", startedCount)
	}
}

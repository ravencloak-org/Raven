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
	"github.com/ravencloak-org/Raven/internal/service"
	"github.com/ravencloak-org/Raven/pkg/apierror"
)

// mockWhatsAppWebhookService implements handler.WhatsAppWebhookServicer.
type mockWhatsAppWebhookService struct {
	verifyWebhookFn       func(mode, token, challenge string) (string, error)
	validateSignatureFn   func(payload []byte, sig string) bool
	handleCallStartedFn   func(ctx context.Context, phoneNumberID, callID, from, to, sdpOffer string) (*model.WhatsAppCall, error)
	handleCallConnectedFn func(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error)
	handleCallEndedFn     func(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error)
	setSDPAnswerFn        func(ctx context.Context, orgID, callID, sdpAnswer string) (*model.WhatsAppCall, error)
	getCallFn             func(ctx context.Context, orgID, id string) (*model.WhatsAppCall, error)
	listCallsFn           func(ctx context.Context, orgID string, limit, offset int) (*model.WhatsAppCallListResponse, error)
}

func (m *mockWhatsAppWebhookService) VerifyWebhook(mode, token, challenge string) (string, error) {
	if m.verifyWebhookFn != nil {
		return m.verifyWebhookFn(mode, token, challenge)
	}
	return challenge, nil
}

func (m *mockWhatsAppWebhookService) ValidateSignature(payload []byte, sig string) bool {
	if m.validateSignatureFn != nil {
		return m.validateSignatureFn(payload, sig)
	}
	return true
}

func (m *mockWhatsAppWebhookService) HandleCallStarted(ctx context.Context, phoneNumberID, callID, from, to, sdpOffer string) (*model.WhatsAppCall, error) {
	if m.handleCallStartedFn != nil {
		return m.handleCallStartedFn(ctx, phoneNumberID, callID, from, to, sdpOffer)
	}
	return &model.WhatsAppCall{}, nil
}

func (m *mockWhatsAppWebhookService) HandleCallConnected(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error) {
	if m.handleCallConnectedFn != nil {
		return m.handleCallConnectedFn(ctx, phoneNumberID, callID)
	}
	return &model.WhatsAppCall{}, nil
}

func (m *mockWhatsAppWebhookService) HandleCallEnded(ctx context.Context, phoneNumberID, callID string) (*model.WhatsAppCall, error) {
	if m.handleCallEndedFn != nil {
		return m.handleCallEndedFn(ctx, phoneNumberID, callID)
	}
	return &model.WhatsAppCall{}, nil
}

func (m *mockWhatsAppWebhookService) SetSDPAnswer(ctx context.Context, orgID, callID, sdpAnswer string) (*model.WhatsAppCall, error) {
	if m.setSDPAnswerFn != nil {
		return m.setSDPAnswerFn(ctx, orgID, callID, sdpAnswer)
	}
	return &model.WhatsAppCall{}, nil
}

func (m *mockWhatsAppWebhookService) GetCall(ctx context.Context, orgID, id string) (*model.WhatsAppCall, error) {
	if m.getCallFn != nil {
		return m.getCallFn(ctx, orgID, id)
	}
	return &model.WhatsAppCall{}, nil
}

func (m *mockWhatsAppWebhookService) ListCalls(ctx context.Context, orgID string, limit, offset int) (*model.WhatsAppCallListResponse, error) {
	if m.listCallsFn != nil {
		return m.listCallsFn(ctx, orgID, limit, offset)
	}
	return &model.WhatsAppCallListResponse{Calls: []model.WhatsAppCall{}}, nil
}

func newWhatsAppWebhookRouter(svc handler.WhatsAppWebhookServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())

	// Public webhook routes (no auth).
	r.GET("/webhooks/whatsapp", handler.NewWhatsAppWebhookHandler(svc).Verify)
	r.POST("/webhooks/whatsapp", handler.NewWhatsAppWebhookHandler(svc).Receive)

	return r
}

func newWhatsAppCallRouter(svc handler.WhatsAppWebhookServicer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyOrgID), "org-test")
		c.Next()
	})

	h := handler.NewWhatsAppWebhookHandler(svc)
	r.GET("/api/v1/orgs/:org_id/whatsapp/calls", h.ListCalls)
	r.GET("/api/v1/orgs/:org_id/whatsapp/calls/:call_id", h.GetCall)
	r.POST("/api/v1/orgs/:org_id/whatsapp/calls/:call_id/answer", h.SendSDPAnswer)
	return r
}

// --- Webhook Verification Tests ---

func TestWhatsAppVerify_Success(t *testing.T) {
	svc := &mockWhatsAppWebhookService{
		verifyWebhookFn: func(mode, token, challenge string) (string, error) {
			if mode != "subscribe" {
				t.Errorf("mode = %q, want 'subscribe'", mode)
			}
			if token != "my-token" {
				t.Errorf("token = %q, want 'my-token'", token)
			}
			if challenge != "challenge-123" {
				t.Errorf("challenge = %q, want 'challenge-123'", challenge)
			}
			return challenge, nil
		},
	}

	r := newWhatsAppWebhookRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet,
		"/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=my-token&hub.challenge=challenge-123",
		nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.String() != "challenge-123" {
		t.Errorf("body = %q, want 'challenge-123'", w.Body.String())
	}
}

func TestWhatsAppVerify_BadToken_Returns401(t *testing.T) {
	svc := &mockWhatsAppWebhookService{
		verifyWebhookFn: func(_, _, _ string) (string, error) {
			return "", service.ErrWebhookTokenMismatch
		},
	}

	r := newWhatsAppWebhookRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet,
		"/webhooks/whatsapp?hub.mode=subscribe&hub.verify_token=wrong&hub.challenge=c",
		nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWhatsAppVerify_BadMode_Returns400(t *testing.T) {
	svc := &mockWhatsAppWebhookService{
		verifyWebhookFn: func(_, _, _ string) (string, error) {
			return "", service.ErrWebhookInvalidMode
		},
	}

	r := newWhatsAppWebhookRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet,
		"/webhooks/whatsapp?hub.mode=unsubscribe&hub.verify_token=t&hub.challenge=c",
		nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Signature Validation Tests ---

func TestWhatsAppReceive_InvalidSignature_Returns401(t *testing.T) {
	svc := &mockWhatsAppWebhookService{
		validateSignatureFn: func(_ []byte, _ string) bool {
			return false
		},
	}

	r := newWhatsAppWebhookRouter(svc)
	body := `{"object":"whatsapp_business_account","entry":[]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWhatsAppReceive_ValidSignature_Returns200(t *testing.T) {
	svc := &mockWhatsAppWebhookService{
		validateSignatureFn: func(_ []byte, _ string) bool {
			return true
		},
	}

	r := newWhatsAppWebhookRouter(svc)
	body := `{"object":"whatsapp_business_account","entry":[]}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=abc")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Call Event Parsing Tests ---

func TestWhatsAppReceive_CallStartedEvent(t *testing.T) {
	var calledWith struct {
		phoneNumberID, callID, from, to, sdpOffer string
	}

	svc := &mockWhatsAppWebhookService{
		handleCallStartedFn: func(_ context.Context, phoneNumberID, callID, from, to, sdpOffer string) (*model.WhatsAppCall, error) {
			calledWith.phoneNumberID = phoneNumberID
			calledWith.callID = callID
			calledWith.from = from
			calledWith.to = to
			calledWith.sdpOffer = sdpOffer
			return &model.WhatsAppCall{CallID: callID, State: model.WhatsAppCallStateRinging}, nil
		},
	}

	payload := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{
			{
				"id": "WHATSAPP_BUSINESS_ACCOUNT_ID",
				"changes": []map[string]any{
					{
						"field": "calls",
						"value": map[string]any{
							"messaging_product": "whatsapp",
							"metadata": map[string]any{
								"display_phone_number": "+1234567890",
								"phone_number_id":      "123456",
							},
							"calls": []map[string]any{
								{
									"id":        "call-abc",
									"from":      "+15551234567",
									"to":        "+15559876543",
									"type":      "voice",
									"status":    "ringing",
									"sdp_offer": "v=0\r\no=...",
								},
							},
						},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	r := newWhatsAppWebhookRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if calledWith.phoneNumberID != "123456" {
		t.Errorf("phoneNumberID = %q, want '123456'", calledWith.phoneNumberID)
	}
	if calledWith.callID != "call-abc" {
		t.Errorf("callID = %q, want 'call-abc'", calledWith.callID)
	}
	if calledWith.from != "+15551234567" {
		t.Errorf("from = %q, want '+15551234567'", calledWith.from)
	}
	if calledWith.sdpOffer != "v=0\r\no=..." {
		t.Errorf("sdpOffer = %q, want 'v=0\\r\\no=...'", calledWith.sdpOffer)
	}
}

func TestWhatsAppReceive_CallConnectedEvent(t *testing.T) {
	connected := false
	svc := &mockWhatsAppWebhookService{
		handleCallConnectedFn: func(_ context.Context, _, callID string) (*model.WhatsAppCall, error) {
			connected = true
			if callID != "call-xyz" {
				t.Errorf("callID = %q, want 'call-xyz'", callID)
			}
			return &model.WhatsAppCall{CallID: callID, State: model.WhatsAppCallStateConnected}, nil
		},
	}

	payload := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{
			{
				"id": "BIZ_ID",
				"changes": []map[string]any{
					{
						"field": "calls",
						"value": map[string]any{
							"metadata": map[string]any{
								"phone_number_id": "123",
							},
							"calls": []map[string]any{
								{
									"id":     "call-xyz",
									"from":   "+1",
									"status": "connected",
								},
							},
						},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	r := newWhatsAppWebhookRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if !connected {
		t.Error("HandleCallConnected was not called")
	}
}

func TestWhatsAppReceive_CallEndedEvent(t *testing.T) {
	ended := false
	svc := &mockWhatsAppWebhookService{
		handleCallEndedFn: func(_ context.Context, _, callID string) (*model.WhatsAppCall, error) {
			ended = true
			return &model.WhatsAppCall{CallID: callID, State: model.WhatsAppCallStateEnded}, nil
		},
	}

	payload := map[string]any{
		"object": "whatsapp_business_account",
		"entry": []map[string]any{
			{
				"id": "BIZ_ID",
				"changes": []map[string]any{
					{
						"field": "calls",
						"value": map[string]any{
							"metadata": map[string]any{
								"phone_number_id": "123",
							},
							"calls": []map[string]any{
								{
									"id":     "call-end",
									"from":   "+1",
									"status": "ended",
								},
							},
						},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	r := newWhatsAppWebhookRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if !ended {
		t.Error("HandleCallEnded was not called")
	}
}

func TestWhatsAppReceive_InvalidPayload_Returns400(t *testing.T) {
	svc := &mockWhatsAppWebhookService{}
	r := newWhatsAppWebhookRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/webhooks/whatsapp", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Authenticated Call API Tests ---

func TestWhatsAppGetCall_Success(t *testing.T) {
	now := time.Now()
	sdp := "v=0\r\no=..."
	svc := &mockWhatsAppWebhookService{
		getCallFn: func(_ context.Context, orgID, id string) (*model.WhatsAppCall, error) {
			if orgID != "org-test" {
				t.Errorf("orgID = %q, want 'org-test'", orgID)
			}
			return &model.WhatsAppCall{
				ID:            id,
				OrgID:         orgID,
				PhoneNumberID: "123",
				CallID:        "call-1",
				From:          "+1",
				To:            "+2",
				State:         model.WhatsAppCallStateRinging,
				SDPOffer:      &sdp,
				CreatedAt:     now,
				UpdatedAt:     now,
			}, nil
		},
	}

	r := newWhatsAppCallRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/whatsapp/calls/uuid-1", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.WhatsAppCallResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Call.CallID != "call-1" {
		t.Errorf("call_id = %q, want 'call-1'", resp.Call.CallID)
	}
}

func TestWhatsAppGetCall_NotFound(t *testing.T) {
	svc := &mockWhatsAppWebhookService{
		getCallFn: func(_ context.Context, _, _ string) (*model.WhatsAppCall, error) {
			return nil, service.ErrWebhookCallNotFound
		},
	}

	r := newWhatsAppCallRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/whatsapp/calls/missing", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWhatsAppListCalls_Success(t *testing.T) {
	svc := &mockWhatsAppWebhookService{
		listCallsFn: func(_ context.Context, orgID string, limit, offset int) (*model.WhatsAppCallListResponse, error) {
			return &model.WhatsAppCallListResponse{
				Calls:  []model.WhatsAppCall{{ID: "c1", OrgID: orgID, CallID: "call-1"}},
				Total:  1,
				Limit:  limit,
				Offset: offset,
			}, nil
		},
	}

	r := newWhatsAppCallRouter(svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/orgs/org-test/whatsapp/calls?limit=10&offset=0", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.WhatsAppCallListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Total)
	}
}

func TestWhatsAppSendSDPAnswer_Success(t *testing.T) {
	sdp := "v=0\r\nanswer..."
	svc := &mockWhatsAppWebhookService{
		setSDPAnswerFn: func(_ context.Context, orgID, callID, sdpAnswer string) (*model.WhatsAppCall, error) {
			if orgID != "org-test" {
				t.Errorf("orgID = %q, want 'org-test'", orgID)
			}
			if callID != "call-abc" {
				t.Errorf("callID = %q, want 'call-abc'", callID)
			}
			if sdpAnswer != sdp {
				t.Errorf("sdpAnswer mismatch")
			}
			return &model.WhatsAppCall{
				CallID:    callID,
				SDPAnswer: &sdpAnswer,
				State:     model.WhatsAppCallStateRinging,
			}, nil
		},
	}

	r := newWhatsAppCallRouter(svc)
	body := `{"sdp_answer":"v=0\r\nanswer..."}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/whatsapp/calls/call-abc/answer", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWhatsAppSendSDPAnswer_MissingBody_Returns400(t *testing.T) {
	svc := &mockWhatsAppWebhookService{}
	r := newWhatsAppCallRouter(svc)
	body := `{}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/whatsapp/calls/call-abc/answer", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWhatsAppSendSDPAnswer_MissingOrgID_Returns401(t *testing.T) {
	svc := &mockWhatsAppWebhookService{}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apierror.ErrorHandler())
	h := handler.NewWhatsAppWebhookHandler(svc)
	r.POST("/api/v1/orgs/:org_id/whatsapp/calls/:call_id/answer", h.SendSDPAnswer)

	body := `{"sdp_answer":"v=0..."}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/orgs/org-test/whatsapp/calls/call-abc/answer", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Service-level Signature Validation ---

func TestHMACSHA256Signature(t *testing.T) {
	secret := "test-app-secret"
	payload := []byte(`{"test":"data"}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Test valid signature
	mac2 := hmac.New(sha256.New, []byte(secret))
	mac2.Write(payload)
	expected := mac2.Sum(nil)

	received, err := hex.DecodeString(expectedSig[7:])
	if err != nil {
		t.Fatalf("hex decode: %v", err)
	}
	if !hmac.Equal(expected, received) {
		t.Error("expected signatures to match")
	}

	// Test invalid signature
	badSig := "sha256=0000000000000000000000000000000000000000000000000000000000000000"
	badReceived, _ := hex.DecodeString(badSig[7:])
	if hmac.Equal(expected, badReceived) {
		t.Error("expected signatures not to match")
	}
}

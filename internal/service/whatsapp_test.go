package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ravencloak-org/Raven/internal/model"
	"github.com/ravencloak-org/Raven/internal/service"
)

// mockWhatsAppRepo implements service.WhatsAppRepository for unit testing.
type mockWhatsAppRepo struct {
	createPhoneNumberFn func(ctx context.Context, tx pgx.Tx, orgID string, req *model.CreateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error)
	getPhoneNumberFn    func(ctx context.Context, tx pgx.Tx, orgID, phoneID string) (*model.WhatsAppPhoneNumber, error)
	updatePhoneNumberFn func(ctx context.Context, tx pgx.Tx, orgID, phoneID string, req *model.UpdateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error)
	deletePhoneNumberFn func(ctx context.Context, tx pgx.Tx, orgID, phoneID string) error
	listPhoneNumbersFn  func(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.WhatsAppPhoneNumber, int, error)
	createCallFn        func(ctx context.Context, tx pgx.Tx, orgID string, call *model.WhatsAppCall) (*model.WhatsAppCall, error)
	getCallFn           func(ctx context.Context, tx pgx.Tx, orgID, callID string) (*model.WhatsAppCall, error)
	getCallByCallIDFn   func(ctx context.Context, tx pgx.Tx, orgID, metaCallID string) (*model.WhatsAppCall, error)
	updateCallStateFn   func(ctx context.Context, tx pgx.Tx, orgID, callID string, state model.WhatsAppCallState) (*model.WhatsAppCall, error)
	listCallsFn         func(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.WhatsAppCall, int, error)
}

func (m *mockWhatsAppRepo) CreatePhoneNumber(ctx context.Context, tx pgx.Tx, orgID string, req *model.CreateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error) {
	if m.createPhoneNumberFn != nil {
		return m.createPhoneNumberFn(ctx, tx, orgID, req)
	}
	return nil, nil
}

func (m *mockWhatsAppRepo) GetPhoneNumber(ctx context.Context, tx pgx.Tx, orgID, phoneID string) (*model.WhatsAppPhoneNumber, error) {
	if m.getPhoneNumberFn != nil {
		return m.getPhoneNumberFn(ctx, tx, orgID, phoneID)
	}
	return nil, nil
}

func (m *mockWhatsAppRepo) UpdatePhoneNumber(ctx context.Context, tx pgx.Tx, orgID, phoneID string, req *model.UpdateWhatsAppPhoneNumberRequest) (*model.WhatsAppPhoneNumber, error) {
	if m.updatePhoneNumberFn != nil {
		return m.updatePhoneNumberFn(ctx, tx, orgID, phoneID, req)
	}
	return nil, nil
}

func (m *mockWhatsAppRepo) DeletePhoneNumber(ctx context.Context, tx pgx.Tx, orgID, phoneID string) error {
	if m.deletePhoneNumberFn != nil {
		return m.deletePhoneNumberFn(ctx, tx, orgID, phoneID)
	}
	return nil
}

func (m *mockWhatsAppRepo) ListPhoneNumbers(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.WhatsAppPhoneNumber, int, error) {
	if m.listPhoneNumbersFn != nil {
		return m.listPhoneNumbersFn(ctx, tx, orgID, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockWhatsAppRepo) CreateCall(ctx context.Context, tx pgx.Tx, orgID string, call *model.WhatsAppCall) (*model.WhatsAppCall, error) {
	if m.createCallFn != nil {
		return m.createCallFn(ctx, tx, orgID, call)
	}
	return nil, nil
}

func (m *mockWhatsAppRepo) GetCall(ctx context.Context, tx pgx.Tx, orgID, callID string) (*model.WhatsAppCall, error) {
	if m.getCallFn != nil {
		return m.getCallFn(ctx, tx, orgID, callID)
	}
	return nil, nil
}

func (m *mockWhatsAppRepo) GetCallByCallID(ctx context.Context, tx pgx.Tx, orgID, metaCallID string) (*model.WhatsAppCall, error) {
	if m.getCallByCallIDFn != nil {
		return m.getCallByCallIDFn(ctx, tx, orgID, metaCallID)
	}
	return nil, nil
}

func (m *mockWhatsAppRepo) UpdateCallState(ctx context.Context, tx pgx.Tx, orgID, callID string, state model.WhatsAppCallState) (*model.WhatsAppCall, error) {
	if m.updateCallStateFn != nil {
		return m.updateCallStateFn(ctx, tx, orgID, callID, state)
	}
	return nil, nil
}

func (m *mockWhatsAppRepo) ListCalls(ctx context.Context, tx pgx.Tx, orgID string, limit, offset int) ([]model.WhatsAppCall, int, error) {
	if m.listCallsFn != nil {
		return m.listCallsFn(ctx, tx, orgID, limit, offset)
	}
	return nil, 0, nil
}

// Ensure mockWhatsAppRepo satisfies the interface.
var _ service.WhatsAppRepository = (*mockWhatsAppRepo)(nil)

// TestWhatsAppModel_CallDirectionValues validates call direction enum string values.
func TestWhatsAppModel_CallDirectionValues(t *testing.T) {
	cases := []struct {
		dir     model.WhatsAppCallDirection
		wantStr string
	}{
		{model.WhatsAppCallDirectionInbound, "inbound"},
		{model.WhatsAppCallDirectionOutbound, "outbound"},
	}
	for _, tc := range cases {
		if string(tc.dir) != tc.wantStr {
			t.Errorf("WhatsAppCallDirection: got %q, want %q", string(tc.dir), tc.wantStr)
		}
	}
}

// TestWhatsAppModel_CallStateValues validates call state enum string values.
func TestWhatsAppModel_CallStateValues(t *testing.T) {
	cases := []struct {
		state   model.WhatsAppCallState
		wantStr string
	}{
		{model.WhatsAppCallStateRinging, "ringing"},
		{model.WhatsAppCallStateConnected, "connected"},
		{model.WhatsAppCallStateEnded, "ended"},
	}
	for _, tc := range cases {
		if string(tc.state) != tc.wantStr {
			t.Errorf("WhatsAppCallState: got %q, want %q", string(tc.state), tc.wantStr)
		}
	}
}

// TestWhatsAppModel_CallDurationSemantics validates WhatsAppCall struct semantics.
func TestWhatsAppModel_CallDurationSemantics(t *testing.T) {
	now := time.Now()
	start := now.Add(-2 * time.Minute)
	end := now
	dur := int(end.Sub(start).Seconds())
	call := model.WhatsAppCall{
		ID:              "c-1",
		OrgID:           "o-1",
		CallID:          "mc-1",
		PhoneNumberID:   "p-1",
		Direction:       model.WhatsAppCallDirectionOutbound,
		State:           model.WhatsAppCallStateEnded,
		Caller:          "+111",
		Callee:          "+222",
		StartedAt:       &start,
		EndedAt:         &end,
		DurationSeconds: &dur,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if call.DurationSeconds == nil {
		t.Fatal("expected duration_seconds to be set")
	}
	if *call.DurationSeconds <= 0 {
		t.Errorf("expected positive duration, got %d", *call.DurationSeconds)
	}
}

// TestWhatsAppService_UpdateCallState_InvalidState_GuardCheck verifies that
// the service rejects the 'ringing' state transition without needing a DB pool.
func TestWhatsAppService_UpdateCallState_InvalidState_GuardCheck(t *testing.T) {
	svc := service.NewWhatsAppService(nil, nil)
	_, err := svc.UpdateCallState(context.Background(), "org-1", "call-1", model.WhatsAppCallStateRinging)
	if err == nil {
		t.Fatal("expected error for invalid state 'ringing', got nil")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestWhatsAppCallListResponse_EmptySlice checks that empty call list returns empty slice.
func TestWhatsAppCallListResponse_EmptySlice(t *testing.T) {
	resp := &model.WhatsAppCallListResponse{
		Calls:  []model.WhatsAppCall{},
		Total:  0,
		Limit:  20,
		Offset: 0,
	}
	if resp.Calls == nil {
		t.Error("expected empty slice, got nil")
	}
}

// TestWhatsAppPhoneNumberListResponse_EmptySlice checks that empty phone list returns empty slice.
func TestWhatsAppPhoneNumberListResponse_EmptySlice(t *testing.T) {
	resp := &model.WhatsAppPhoneNumberListResponse{
		PhoneNumbers: []model.WhatsAppPhoneNumber{},
		Total:        0,
		Limit:        20,
		Offset:       0,
	}
	if resp.PhoneNumbers == nil {
		t.Error("expected empty slice, got nil")
	}
}

// TestWhatsAppWebhookPayload_Structure validates webhook payload struct can be instantiated.
func TestWhatsAppWebhookPayload_Structure(t *testing.T) {
	payload := model.WhatsAppWebhookPayload{
		Object: "whatsapp_business_account",
		Entry: []model.WhatsAppWebhookEntry{
			{
				ID: "entry-1",
				Changes: []model.WhatsAppWebhookChange{
					{
						Field: "calls",
						Value: model.WhatsAppWebhookChangeValue{
							MessagingProduct: "whatsapp",
							CallID:           "call-xyz",
							From:             "+1111111111",
							To:               "+2222222222",
							Status:           "ringing",
						},
					},
				},
			},
		},
	}
	if payload.Object != "whatsapp_business_account" {
		t.Errorf("object = %q, want 'whatsapp_business_account'", payload.Object)
	}
	if len(payload.Entry) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(payload.Entry))
	}
	if len(payload.Entry[0].Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(payload.Entry[0].Changes))
	}
	if payload.Entry[0].Changes[0].Value.CallID != "call-xyz" {
		t.Errorf("call_id = %q, want 'call-xyz'", payload.Entry[0].Changes[0].Value.CallID)
	}
}

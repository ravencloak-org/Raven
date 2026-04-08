package model

import "time"

// WhatsAppCallDirection enumerates the direction of a WhatsApp call.
type WhatsAppCallDirection string

// Supported call directions.
const (
	WhatsAppCallDirectionInbound  WhatsAppCallDirection = "inbound"
	WhatsAppCallDirectionOutbound WhatsAppCallDirection = "outbound"
)

// WhatsAppCallState enumerates the lifecycle states of a WhatsApp call.
type WhatsAppCallState string

// Supported call states.
const (
	WhatsAppCallStateRinging   WhatsAppCallState = "ringing"
	WhatsAppCallStateConnected WhatsAppCallState = "connected"
	WhatsAppCallStateEnded     WhatsAppCallState = "ended"
)

// WhatsAppPhoneNumber represents a provisioned WhatsApp Business phone number.
type WhatsAppPhoneNumber struct {
	ID          string    `json:"id"`
	OrgID       string    `json:"org_id"`
	PhoneNumber string    `json:"phone_number"`
	DisplayName string    `json:"display_name"`
	WABAID      string    `json:"waba_id"`
	Verified    bool      `json:"verified"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WhatsAppCall represents a single WhatsApp Business API call record.
type WhatsAppCall struct {
	ID              string                `json:"id"`
	OrgID           string                `json:"org_id"`
	CallID          string                `json:"call_id"`
	PhoneNumberID   string                `json:"phone_number_id"`
	Direction       WhatsAppCallDirection `json:"direction"`
	State           WhatsAppCallState     `json:"state"`
	Caller          string                `json:"caller"`
	Callee          string                `json:"callee"`
	From            string                `json:"from,omitempty"`
	To              string                `json:"to,omitempty"`
	SDPOffer        *string               `json:"sdp_offer,omitempty"   db:"sdp_offer"`
	SDPAnswer       *string               `json:"sdp_answer,omitempty"  db:"sdp_answer"`
	StartedAt       *time.Time            `json:"started_at,omitempty"`
	EndedAt         *time.Time            `json:"ended_at,omitempty"`
	DurationSeconds *int                  `json:"duration_seconds,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}

// --- Request / Response types ---

// CreateWhatsAppPhoneNumberRequest is the payload for POST /whatsapp/phone-numbers.
type CreateWhatsAppPhoneNumberRequest struct {
	PhoneNumber string `json:"phone_number" binding:"required,max=20"`
	DisplayName string `json:"display_name" binding:"omitempty,max=255"`
	WABAID      string `json:"waba_id" binding:"required,max=64"`
}

// UpdateWhatsAppPhoneNumberRequest is the payload for PUT /whatsapp/phone-numbers/:id.
type UpdateWhatsAppPhoneNumberRequest struct {
	DisplayName *string `json:"display_name,omitempty" binding:"omitempty,max=255"`
	Verified    *bool   `json:"verified,omitempty"`
}

// WhatsAppPhoneNumberListResponse is the paginated list of phone numbers.
type WhatsAppPhoneNumberListResponse struct {
	PhoneNumbers []WhatsAppPhoneNumber `json:"phone_numbers"`
	Total        int                   `json:"total"`
	Limit        int                   `json:"limit"`
	Offset       int                   `json:"offset"`
}

// InitiateWhatsAppCallRequest is the payload for POST /whatsapp/calls.
type InitiateWhatsAppCallRequest struct {
	PhoneNumberID string `json:"phone_number_id" binding:"required,uuid"`
	Callee        string `json:"callee" binding:"required,max=20"`
}

// UpdateWhatsAppCallStateRequest is the payload for PATCH /whatsapp/calls/:id.
type UpdateWhatsAppCallStateRequest struct {
	State WhatsAppCallState `json:"state" binding:"required"`
}

// WhatsAppCallListResponse is the paginated list of calls.
type WhatsAppCallListResponse struct {
	Calls  []WhatsAppCall `json:"calls"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// WhatsAppCallResponse wraps a single WhatsAppCall for JSON API responses.
type WhatsAppCallResponse struct {
	Call WhatsAppCall `json:"call"`
}

// SendSDPAnswerRequest is the payload for POST /whatsapp/calls/:call_id/answer.
type SendSDPAnswerRequest struct {
	SDPAnswer string `json:"sdp_answer" binding:"required"`
}

// --- Webhook payload types (Meta Graph API) ---

// WhatsAppWebhookPayload is the top-level structure of a Meta webhook delivery.
type WhatsAppWebhookPayload struct {
	Object string                 `json:"object"`
	Entry  []WhatsAppWebhookEntry `json:"entry"`
}

// WhatsAppWebhookEntry represents a single entry in the webhook payload.
type WhatsAppWebhookEntry struct {
	ID      string                  `json:"id"`
	Changes []WhatsAppWebhookChange `json:"changes"`
}

// WhatsAppWebhookChange represents a single change within an entry.
type WhatsAppWebhookChange struct {
	Field string                     `json:"field"`
	Value WhatsAppWebhookChangeValue `json:"value"`
}

// WhatsAppWebhookChangeValue holds the call-related data from Meta's webhook.
type WhatsAppWebhookChangeValue struct {
	MessagingProduct string `json:"messaging_product"`
	CallID           string `json:"call_id,omitempty"`
	From             string `json:"from,omitempty"`
	To               string `json:"to,omitempty"`
	Status           string `json:"status,omitempty"`
	Timestamp        string `json:"timestamp,omitempty"`
}

package model

// MetaWebhookPayload is the top-level structure Meta sends to webhooks.
type MetaWebhookPayload struct {
	Object string            `json:"object"`
	Entry  []MetaWebhookEntry `json:"entry"`
}

// MetaWebhookEntry represents a single entry in a Meta webhook notification.
type MetaWebhookEntry struct {
	ID      string              `json:"id"`
	Changes []MetaWebhookChange `json:"changes"`
}

// MetaWebhookChange contains a field identifier and its changed value.
type MetaWebhookChange struct {
	Value MetaWebhookValue `json:"value"`
	Field string           `json:"field"`
}

// MetaWebhookValue contains the actual event data for a change.
type MetaWebhookValue struct {
	MessagingProduct string          `json:"messaging_product"`
	Metadata         MetaMetadata    `json:"metadata"`
	Calls            []MetaCallEvent `json:"calls,omitempty"`
}

// MetaMetadata contains identifying metadata about the WhatsApp Business account.
type MetaMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

// MetaCallEvent represents a single call event delivered via the Meta Graph API webhook.
type MetaCallEvent struct {
	ID        string `json:"id"`                  // Meta call ID
	From      string `json:"from"`                // caller phone number
	To        string `json:"to"`                  // callee phone number
	Status    string `json:"status"`              // "ringing", "answered", "ended"
	SDPOffer  string `json:"sdp_offer,omitempty"`   // SDP offer (present on "ringing")
	Timestamp string `json:"timestamp"`
}

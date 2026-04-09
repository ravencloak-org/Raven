package telemetry

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/ravencloak-org/Raven/internal/telemetry"

// Metrics holds pre-created OTEL instruments for business-level observability.
// All methods are safe for concurrent use.  When the global MeterProvider is
// a no-op (OTel not configured) the instruments silently discard data.
type Metrics struct {
	// HTTP
	httpRequestsTotal metric.Int64Counter

	// Chat completions
	chatCompletionsTotal  metric.Int64Counter
	chatTokensTotal       metric.Int64Counter
	chatCompletionLatency metric.Float64Histogram

	// Voice sessions
	voiceSessionsCreated  metric.Int64Counter
	voiceSessionsActive   metric.Int64UpDownCounter
	voiceTurnsTotal       metric.Int64Counter

	// WhatsApp
	whatsappCallsTotal    metric.Int64Counter
	whatsappMessagesTotal metric.Int64Counter
}

// NewMetrics creates and registers all business metric instruments.
// It never returns an error; creation failures are logged and the
// corresponding instrument is left nil (methods check before recording).
func NewMetrics() *Metrics {
	meter := otel.Meter(meterName)
	m := &Metrics{}

	var err error

	// --- HTTP ---
	m.httpRequestsTotal, err = meter.Int64Counter("raven.http.requests.total",
		metric.WithDescription("Total HTTP requests by method, route and status"),
		metric.WithUnit("{request}"),
	)
	logMetricErr("raven.http.requests.total", err)

	// --- Chat ---
	m.chatCompletionsTotal, err = meter.Int64Counter("raven.chat.completions.total",
		metric.WithDescription("Total chat completion requests"),
		metric.WithUnit("{completion}"),
	)
	logMetricErr("raven.chat.completions.total", err)

	m.chatTokensTotal, err = meter.Int64Counter("raven.chat.tokens.total",
		metric.WithDescription("Total tokens generated in chat completions"),
		metric.WithUnit("{token}"),
	)
	logMetricErr("raven.chat.tokens.total", err)

	m.chatCompletionLatency, err = meter.Float64Histogram("raven.chat.completion.latency",
		metric.WithDescription("Latency of chat completion requests"),
		metric.WithUnit("ms"),
	)
	logMetricErr("raven.chat.completion.latency", err)

	// --- Voice ---
	m.voiceSessionsCreated, err = meter.Int64Counter("raven.voice.sessions.created",
		metric.WithDescription("Total voice sessions created"),
		metric.WithUnit("{session}"),
	)
	logMetricErr("raven.voice.sessions.created", err)

	m.voiceSessionsActive, err = meter.Int64UpDownCounter("raven.voice.sessions.active",
		metric.WithDescription("Currently active voice sessions"),
		metric.WithUnit("{session}"),
	)
	logMetricErr("raven.voice.sessions.active", err)

	m.voiceTurnsTotal, err = meter.Int64Counter("raven.voice.turns.total",
		metric.WithDescription("Total voice turns appended"),
		metric.WithUnit("{turn}"),
	)
	logMetricErr("raven.voice.turns.total", err)

	// --- WhatsApp ---
	m.whatsappCallsTotal, err = meter.Int64Counter("raven.whatsapp.calls.total",
		metric.WithDescription("Total WhatsApp calls initiated"),
		metric.WithUnit("{call}"),
	)
	logMetricErr("raven.whatsapp.calls.total", err)

	m.whatsappMessagesTotal, err = meter.Int64Counter("raven.whatsapp.messages.total",
		metric.WithDescription("Total WhatsApp messages processed"),
		metric.WithUnit("{message}"),
	)
	logMetricErr("raven.whatsapp.messages.total", err)

	return m
}

func logMetricErr(name string, err error) {
	if err != nil {
		slog.Warn("failed to create metric instrument", "name", name, "error", err)
	}
}

// --- HTTP recording helpers ---

// RecordHTTPRequest increments the HTTP request counter.
func (m *Metrics) RecordHTTPRequest(ctx context.Context, method, route string, status int) {
	if m.httpRequestsTotal == nil {
		return
	}
	m.httpRequestsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("http.method", method),
			attribute.String("http.route", route),
			attribute.Int("http.status_code", status),
		),
	)
}

// --- Chat recording helpers ---

// RecordChatCompletion records a chat completion event.
func (m *Metrics) RecordChatCompletion(ctx context.Context, orgID, kbID, model string) {
	if m.chatCompletionsTotal == nil {
		return
	}
	m.chatCompletionsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("org_id", orgID),
			attribute.String("kb_id", kbID),
			attribute.String("model", model),
		),
	)
}

// RecordChatTokens records token usage for a completion.
func (m *Metrics) RecordChatTokens(ctx context.Context, orgID string, tokens int64) {
	if m.chatTokensTotal == nil {
		return
	}
	m.chatTokensTotal.Add(ctx, tokens,
		metric.WithAttributes(attribute.String("org_id", orgID)),
	)
}

// RecordChatLatency records the latency of a chat completion in milliseconds.
func (m *Metrics) RecordChatLatency(ctx context.Context, orgID string, latencyMs float64) {
	if m.chatCompletionLatency == nil {
		return
	}
	m.chatCompletionLatency.Record(ctx, latencyMs,
		metric.WithAttributes(attribute.String("org_id", orgID)),
	)
}

// --- Voice recording helpers ---

// RecordVoiceSessionCreated increments the voice session created counter.
func (m *Metrics) RecordVoiceSessionCreated(ctx context.Context, orgID string) {
	if m.voiceSessionsCreated == nil {
		return
	}
	m.voiceSessionsCreated.Add(ctx, 1,
		metric.WithAttributes(attribute.String("org_id", orgID)),
	)
}

// RecordVoiceSessionActive adjusts the active voice session gauge (delta: +1 or -1).
func (m *Metrics) RecordVoiceSessionActive(ctx context.Context, orgID string, delta int64) {
	if m.voiceSessionsActive == nil {
		return
	}
	m.voiceSessionsActive.Add(ctx, delta,
		metric.WithAttributes(attribute.String("org_id", orgID)),
	)
}

// RecordVoiceTurn increments the voice turn counter.
func (m *Metrics) RecordVoiceTurn(ctx context.Context, orgID string) {
	if m.voiceTurnsTotal == nil {
		return
	}
	m.voiceTurnsTotal.Add(ctx, 1,
		metric.WithAttributes(attribute.String("org_id", orgID)),
	)
}

// --- WhatsApp recording helpers ---

// RecordWhatsAppCall increments the WhatsApp call counter.
func (m *Metrics) RecordWhatsAppCall(ctx context.Context, orgID, direction string) {
	if m.whatsappCallsTotal == nil {
		return
	}
	m.whatsappCallsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("org_id", orgID),
			attribute.String("direction", direction),
		),
	)
}

// RecordWhatsAppMessage increments the WhatsApp message counter.
func (m *Metrics) RecordWhatsAppMessage(ctx context.Context, orgID, msgType string) {
	if m.whatsappMessagesTotal == nil {
		return
	}
	m.whatsappMessagesTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("org_id", orgID),
			attribute.String("type", msgType),
		),
	)
}

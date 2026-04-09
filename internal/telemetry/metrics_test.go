package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func setupMetricReader(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	return reader
}

func collectMetricNames(t *testing.T, reader *sdkmetric.ManualReader) map[string]bool {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}
	names := make(map[string]bool)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names[m.Name] = true
		}
	}
	return names
}

func TestNewMetrics_CreatesAllInstruments(t *testing.T) {
	reader := setupMetricReader(t)
	m := NewMetrics()

	// Record one of each to ensure instruments are exercised.
	ctx := context.Background()
	m.RecordHTTPRequest(ctx, "GET", "/healthz", 200)
	m.RecordChatCompletion(ctx, "org-1", "kb-1", "gpt-4")
	m.RecordChatTokens(ctx, "org-1", 42)
	m.RecordChatLatency(ctx, "org-1", 123.4)
	m.RecordVoiceSessionCreated(ctx, "org-1")
	m.RecordVoiceSessionActive(ctx, "org-1", 1)
	m.RecordVoiceTurn(ctx, "org-1")
	m.RecordWhatsAppCall(ctx, "org-1", "outbound")
	m.RecordWhatsAppMessage(ctx, "org-1", "text")

	names := collectMetricNames(t, reader)

	expected := []string{
		"raven.http.requests.total",
		"raven.chat.completions.total",
		"raven.chat.tokens.total",
		"raven.chat.completion.latency",
		"raven.voice.sessions.created",
		"raven.voice.sessions.active",
		"raven.voice.turns.total",
		"raven.whatsapp.calls.total",
		"raven.whatsapp.messages.total",
	}

	for _, name := range expected {
		if !names[name] {
			t.Errorf("expected metric %q to be recorded", name)
		}
	}
}

func TestMetrics_NilInstruments_NoPanic(t *testing.T) {
	// A zero-value Metrics has all nil instruments; methods must not panic.
	m := &Metrics{}
	ctx := context.Background()

	m.RecordHTTPRequest(ctx, "GET", "/", 200)
	m.RecordChatCompletion(ctx, "", "", "")
	m.RecordChatTokens(ctx, "", 0)
	m.RecordChatLatency(ctx, "", 0)
	m.RecordVoiceSessionCreated(ctx, "")
	m.RecordVoiceSessionActive(ctx, "", 1)
	m.RecordVoiceTurn(ctx, "")
	m.RecordWhatsAppCall(ctx, "", "")
	m.RecordWhatsAppMessage(ctx, "", "")
}

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupTestOTel configures an in-memory span exporter and metric reader so
// tests can inspect what the middleware recorded.
func setupTestOTel(t *testing.T) (*tracetest.InMemoryExporter, *sdkmetric.ManualReader) {
	t.Helper()

	spanExporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(spanExporter),
	)

	metricReader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricReader),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		_ = mp.Shutdown(context.Background())
	})

	return spanExporter, metricReader
}

// newTestRouter returns a Gin engine with the OTel middleware and a single
// GET /healthz route that returns 200.
func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OTelMiddleware())
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestSpanCreatedForRequest(t *testing.T) {
	spanExporter, _ := setupTestOTel(t)

	router := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(w, req)

	spans := spanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span to be recorded")
	}
}

func TestTraceIDHeaderSet(t *testing.T) {
	setupTestOTel(t)

	router := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(w, req)

	traceID := w.Header().Get("X-Trace-ID")
	if traceID == "" {
		t.Fatal("expected X-Trace-ID header to be set")
	}
	if len(traceID) != 32 {
		t.Fatalf("expected 32-char trace ID, got %d chars: %s", len(traceID), traceID)
	}
}

func TestSpanAttributes(t *testing.T) {
	spanExporter, _ := setupTestOTel(t)

	router := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(w, req)

	spans := spanExporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	span := spans[0]

	// Check span name includes method and route.
	if span.Name != "GET /healthz" {
		t.Errorf("expected span name 'GET /healthz', got %q", span.Name)
	}

	// Collect attributes into a map for easy lookup.
	attrs := make(map[string]interface{})
	for _, a := range span.Attributes {
		attrs[string(a.Key)] = a.Value.AsInterface()
	}

	if v, ok := attrs["http.request.method"]; !ok || v != "GET" {
		t.Errorf("expected http.request.method=GET, got %v", v)
	}
	if _, ok := attrs["http.response.status_code"]; !ok {
		t.Error("expected http.response.status_code attribute to be set")
	}
	if _, ok := attrs["http.route"]; !ok {
		t.Error("expected http.route attribute to be set")
	}
}

func TestRequestDurationMetric(t *testing.T) {
	_, metricReader := setupTestOTel(t)

	router := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(w, req)

	var rm metricdata.ResourceMetrics
	if err := metricReader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("failed to collect metrics: %v", err)
	}

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "http.server.request.duration" {
				found = true
			}
		}
	}

	if !found {
		t.Error("expected http.server.request.duration metric to be recorded")
	}
}

func TestNoOpModeNoPanic(t *testing.T) {
	// Use the default no-op providers by creating a fresh tracer/meter
	// provider with no exporters. This simulates the state when OTel is
	// not configured.
	noopTP := sdktrace.NewTracerProvider()
	noopMP := sdkmetric.NewMeterProvider()

	otel.SetTracerProvider(noopTP)
	otel.SetMeterProvider(noopMP)
	t.Cleanup(func() {
		_ = noopTP.Shutdown(context.Background())
		_ = noopMP.Shutdown(context.Background())
	})

	router := newTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	// Must not panic.
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

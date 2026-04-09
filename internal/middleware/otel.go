package middleware

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/ravencloak-org/Raven/internal/telemetry"
)

const instrumentationName = "github.com/ravencloak-org/Raven/internal/middleware"

// OTelMiddleware returns a Gin middleware that creates a span per HTTP request,
// records request duration as a histogram metric, sets the X-Trace-ID
// response header, and emits a structured slog entry with trace_id, org_id
// and latency_ms fields for log aggregation.
//
// When the global TracerProvider is a no-op (i.e. OTel is not configured) the
// middleware still runs but produces zero overhead because the SDK short-
// circuits all recording.
func OTelMiddleware() gin.HandlerFunc {
	tracer := otel.Tracer(instrumentationName)
	meter := otel.Meter(instrumentationName)
	bm := telemetry.NewMetrics()

	// Best-effort histogram; if creation fails we continue without metrics.
	duration, _ := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP server requests"),
		metric.WithUnit("s"),
	)

	return func(c *gin.Context) {
		start := time.Now()

		// Derive a readable span name: "GET /healthz".
		spanName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
		if c.FullPath() == "" {
			spanName = fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)
		}

		ctx, span := tracer.Start(c.Request.Context(), spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(c.Request.Method),
				semconv.URLPath(c.Request.URL.Path),
			),
		)
		defer span.End()

		// Propagate the trace context into the request so downstream
		// handlers and services can join the trace.
		c.Request = c.Request.WithContext(ctx)

		// Expose the trace ID as a response header for easy debugging.
		traceID := ""
		if span.SpanContext().HasTraceID() {
			traceID = span.SpanContext().TraceID().String()
			c.Header("X-Trace-ID", traceID)
		}

		// Process request.
		c.Next()

		// After the request completes record status on the span.
		status := c.Writer.Status()
		span.SetAttributes(
			semconv.HTTPResponseStatusCode(status),
			semconv.HTTPRoute(c.FullPath()),
		)

		elapsed := time.Since(start)
		elapsedSec := elapsed.Seconds()
		latencyMs := float64(elapsed.Milliseconds())
		route := c.FullPath()

		// Record duration metric.
		if duration != nil {
			duration.Record(ctx,
				elapsedSec,
				metric.WithAttributes(
					semconv.HTTPRequestMethodKey.String(c.Request.Method),
					semconv.HTTPRoute(route),
					semconv.HTTPResponseStatusCode(status),
				),
			)
		}

		// Record business-level HTTP request counter.
		bm.RecordHTTPRequest(ctx, c.Request.Method, route, status)

		// Emit a structured log line with trace context for log aggregation.
		// The otelslog bridge automatically correlates trace_id/span_id when
		// the context carries a span.
		orgID, _ := c.Get(string(ContextKeyOrgID))
		orgStr, _ := orgID.(string)

		slog.InfoContext(ctx, "http request",
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.String("route", route),
			slog.Int("status", status),
			slog.Float64("latency_ms", latencyMs),
			slog.String("trace_id", traceID),
			slog.String("org_id", orgStr),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.String("client_ip", c.ClientIP()),
		)
	}
}

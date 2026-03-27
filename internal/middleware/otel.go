package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/ravencloak-org/Raven/internal/middleware"

// OTelMiddleware returns a Gin middleware that creates a span per HTTP request,
// records request duration as a histogram metric, and sets the X-Trace-ID
// response header.
//
// When the global TracerProvider is a no-op (i.e. OTel is not configured) the
// middleware still runs but produces zero overhead because the SDK short-
// circuits all recording.
func OTelMiddleware() gin.HandlerFunc {
	tracer := otel.Tracer(instrumentationName)
	meter := otel.Meter(instrumentationName)

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
		if span.SpanContext().HasTraceID() {
			c.Header("X-Trace-ID", span.SpanContext().TraceID().String())
		}

		// Process request.
		c.Next()

		// After the request completes record status on the span.
		status := c.Writer.Status()
		span.SetAttributes(
			semconv.HTTPResponseStatusCode(status),
			semconv.HTTPRoute(c.FullPath()),
		)

		// Record duration metric.
		elapsed := time.Since(start).Seconds()
		if duration != nil {
			duration.Record(ctx,
				elapsed,
				metric.WithAttributes(
					semconv.HTTPRequestMethodKey.String(c.Request.Method),
					semconv.HTTPRoute(c.FullPath()),
					semconv.HTTPResponseStatusCode(status),
				),
			)
		}
	}
}

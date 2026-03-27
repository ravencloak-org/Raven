// Package telemetry initialises OpenTelemetry tracing and metrics for the
// Raven API.  When no OTLP endpoint is configured the package gracefully
// degrades to no-op providers so the rest of the application works without
// any observable overhead.
package telemetry

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitProvider sets up a TracerProvider and MeterProvider.
//
// If endpoint is non-empty, OTLP gRPC exporters are configured for both traces
// and metrics.  Otherwise no-op providers are used so that callers never need
// to nil-check.
//
// The returned shutdown function flushes pending telemetry and releases
// resources.  It is safe to call even when the providers are no-ops.
func InitProvider(ctx context.Context, serviceName, serviceVersion, endpoint, environment string) (shutdown func(context.Context) error, err error) {
	// Build the OTel resource that describes this service.
	// We use NewSchemaless to avoid schema-URL conflicts with resource.Default().
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			semconv.DeploymentEnvironment(environment),
		),
	)
	if err != nil {
		return nil, err
	}

	// When there is no endpoint we leave the global providers at their
	// defaults (no-op).  Return a no-op shutdown.
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	var shutdownFuncs []func(context.Context) error

	// Helper that aggregates all shutdown functions.
	shutdownAll := func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFuncs {
			if e := fn(ctx); e != nil {
				errs = append(errs, e)
			}
		}
		return errors.Join(errs...)
	}

	// --- Trace exporter ---
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return shutdownAll, err
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, tp.Shutdown)
	otel.SetTracerProvider(tp)

	// --- Metric exporter ---
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return shutdownAll, err
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(res),
	)
	shutdownFuncs = append(shutdownFuncs, mp.Shutdown)
	otel.SetMeterProvider(mp)

	// --- Propagation ---
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return shutdownAll, nil
}

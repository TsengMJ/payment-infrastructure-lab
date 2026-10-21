// Package otel configures tracing and metrics for the process.
// Metrics are produced with the OpenTelemetry API and exposed for Prometheus.
// Traces are exported with OTLP only when OTEL_EXPORTER_OTLP_ENDPOINT is set.
package otel

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/otlptranslator"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Setup installs the global tracer and meter providers.
// The returned handler serves Prometheus text on /metrics.
// Shutdown flushes exporters. Calling it more than once returns an error.
func Setup(ctx context.Context, service string) (http.Handler, func(context.Context) error, error) {
	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", service),
	))
	if err != nil {
		return nil, nil, err
	}

	reg := prometheus.NewRegistry()
	promExporter, err := otelprom.New(
		otelprom.WithRegisterer(reg),
		otelprom.WithTranslationStrategy(otlptranslator.UnderscoreEscapingWithSuffixes),
	)
	if err != nil {
		return nil, nil, err
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)

	var tracerOpts []sdktrace.TracerProviderOption
	tracerOpts = append(tracerOpts, sdktrace.WithResource(res))
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		// The Go SDK also reads this variable and requires a URL scheme.
		// The lab value is host:port. Read it ourselves, then clear it so the
		// SDK does not log a parse error and fall back to TLS on localhost.
		host, insecure := otlpTarget(endpoint)
		_ = os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(host)}
		if insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exp, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, nil, err
		}
		tracerOpts = append(tracerOpts, sdktrace.WithBatcher(exp))
	}
	tracerProvider := sdktrace.NewTracerProvider(tracerOpts...)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	shutdown := func(ctx context.Context) error {
		errMeter := meterProvider.Shutdown(ctx)
		errTracer := tracerProvider.Shutdown(ctx)
		if errMeter != nil {
			return errMeter
		}
		return errTracer
	}
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), shutdown, nil
}

// HTTPMiddleware records a server span and request metrics for each matched route.
// The metric attributes from this middleware are the route template only.
// otelhttp also adds method, status, and a few low-cardinality protocol attributes.
// It does not label metrics with the raw URL, so an id in the path stays out of Prometheus.
func HTTPMiddleware(service string, opts ...otelhttp.Option) func(http.Handler) http.Handler {
	base := []otelhttp.Option{
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return routePattern(r)
		}),
		otelhttp.WithMetricAttributesFn(func(r *http.Request) []attribute.KeyValue {
			return []attribute.KeyValue{attribute.String("http.route", routePattern(r))}
		}),
	}
	return otelhttp.NewMiddleware(service, append(base, opts...)...)
}

func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if pattern := rc.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "unknown"
}

// otlpTarget accepts "127.0.0.1:4317" or "http://127.0.0.1:4317".
// A value without a scheme is an insecure local collector.
func otlpTarget(raw string) (host string, insecure bool) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		return raw, true
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw, true
	}
	return u.Host, u.Scheme != "https"
}

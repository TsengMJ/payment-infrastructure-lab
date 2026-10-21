package otel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestRequestMetricsUseRouteTemplate(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))

	r := chi.NewRouter()
	r.Use(HTTPMiddleware("payment-api", otelhttp.WithMeterProvider(provider)))
	r.Get("/payments/{paymentID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/payments/pay_123", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	var got metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &got); err != nil {
		t.Fatalf("collect: %v", err)
	}
	var sawRoute bool
	for _, attr := range metricAttributes(got) {
		key := string(attr.Key)
		switch key {
		case "payment_id", "merchant_id", "trace_id", "url.path", "url.full", "http.target":
			t.Fatalf("high-cardinality attribute %s=%v", key, attr.Value)
		}
		if attr.Value.AsString() == "pay_123" || attr.Value.AsString() == "/payments/pay_123" {
			t.Fatalf("raw id in attribute %s=%v", key, attr.Value)
		}
		if key == "http.route" {
			sawRoute = true
			if attr.Value.AsString() != "/payments/{paymentID}" {
				t.Fatalf("http.route = %s", attr.Value.AsString())
			}
		}
	}
	if !sawRoute {
		t.Fatal("http.route attribute missing")
	}
}

func metricAttributes(rm metricdata.ResourceMetrics) []attribute.KeyValue {
	var out []attribute.KeyValue
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Histogram[float64]:
				for _, dp := range data.DataPoints {
					out = append(out, dp.Attributes.ToSlice()...)
				}
			case metricdata.Sum[int64]:
				for _, dp := range data.DataPoints {
					out = append(out, dp.Attributes.ToSlice()...)
				}
			case metricdata.Sum[float64]:
				for _, dp := range data.DataPoints {
					out = append(out, dp.Attributes.ToSlice()...)
				}
			}
		}
	}
	return out
}

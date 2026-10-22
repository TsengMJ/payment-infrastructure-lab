package requestid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/otel"
)

func TestRequestIDIsOnHeaderAndSpan(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	r := chi.NewRouter()
	r.Use(otel.HTTPMiddleware("payment-api", otelhttp.WithTracerProvider(provider)))
	r.Use(Middleware)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if FromContext(r.Context()) != "merchant_ref-1" {
			t.Errorf("context = %q", FromContext(r.Context()))
		}
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Request-Id", "merchant_ref-1")
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if got := res.Header().Get("Request-Id"); got != "merchant_ref-1" {
		t.Fatalf("header = %q", got)
	}
	var found bool
	for _, span := range rec.Ended() {
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "request_id" && attr.Value.AsString() == "merchant_ref-1" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("span attribute request_id missing")
	}
}

func TestInvalidRequestIDIsReplaced(t *testing.T) {
	for _, header := range []string{"", "has space", "line\nbreak", strings.Repeat("a", 65), "bad/id"} {
		r := chi.NewRouter()
		r.Use(Middleware)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if FromContext(r.Context()) != w.Header().Get("Request-Id") {
				t.Errorf("context %q header %q", FromContext(r.Context()), w.Header().Get("Request-Id"))
			}
			w.WriteHeader(http.StatusNoContent)
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set("Request-Id", header)
		}
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		got := res.Header().Get("Request-Id")
		if got == header || !strings.HasPrefix(got, "req_") || strings.Contains(got, " ") || strings.Contains(got, "\n") {
			t.Fatalf("header %q became %q", header, got)
		}
	}
}

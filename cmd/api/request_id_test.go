package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/auth"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/log"
)

func TestUnauthorizedLogUsesRequestID(t *testing.T) {
	recSpans := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recSpans))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	var buf bytes.Buffer
	logger := log.New(&buf, "payment-api", slog.LevelInfo)
	token := bearerFixture()
	handler := newHandler(fakePinger{}, logger, &rejectAuth{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/api_keys", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Request-Id", "support-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized || rec.Header().Get("Request-Id") != "support-1" {
		t.Fatalf("status %d header %q body %s", rec.Code, rec.Header().Get("Request-Id"), rec.Body.String())
	}
	if strings.Contains(buf.String(), token) {
		t.Fatalf("log contains the api key: %s", buf.String())
	}
	record := decodeLog(t, &buf)
	if record["request_id"] != "support-1" || record["http.route"] != "/v1/api_keys" || record["trace_id"] == "" {
		t.Fatalf("log = %#v", record)
	}
	status, ok := record["http.status"].(float64)
	if !ok || status != http.StatusUnauthorized {
		t.Fatalf("status = %#v", record["http.status"])
	}
	var found bool
	for _, span := range recSpans.Ended() {
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "request_id" && attr.Value.AsString() == "support-1" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("span attribute request_id missing")
	}
}

type rejectAuth struct{}

func (rejectAuth) Authenticate(context.Context, string) (auth.Identity, error) {
	return auth.Identity{}, auth.ErrUnauthenticated
}

func decodeLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal %s: %v", buf.String(), err)
	}
	return record
}

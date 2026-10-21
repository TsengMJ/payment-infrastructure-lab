package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestRecordHasRequiredFields(t *testing.T) {
	logger, buf := testLogger(t, "payment-api")
	logger.Info("started")

	record := decode(t, buf)
	if record["time"] == "" || record["level"] != "INFO" {
		t.Fatalf("time or level missing: %#v", record)
	}
	if record["service"] != "payment-api" {
		t.Fatalf("service = %#v", record["service"])
	}
	if record["request_id"] != "" || record["trace_id"] != "" {
		t.Fatalf("ids = request %#v trace %#v", record["request_id"], record["trace_id"])
	}
}

func TestRedactsCardNumberAndAuthorization(t *testing.T) {
	logger, buf := testLogger(t, "payment-api")
	logger.Info("charge",
		slog.String("card_number", "4111111111111111"),
		slog.String("currency", "USD"),
		slog.Any("headers", map[string][]string{
			"Authorization": {"Bearer super-secret"},
			"Accept":        {"application/json"},
		}),
		slog.Group("payment",
			slog.String("Card_Number", "4222222222222222"),
			slog.String("amount", "100"),
		),
		slog.Any("body", map[string]any{
			"cvv": "123",
			"nested": map[string]any{
				"api_key": "key-secret",
				"ok":      true,
			},
		}),
	)

	logged := buf.String()
	for _, secret := range []string{"4111111111111111", "Bearer super-secret", "4222222222222222", "key-secret"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("log contains %q: %s", secret, logged)
		}
	}

	record := decode(t, buf)
	if record["card_number"] != "***" || record["currency"] != "USD" {
		t.Fatalf("top-level fields: %#v", record)
	}
	headers := record["headers"].(map[string]any)
	accept, _ := headers["Accept"].([]any)
	if headers["Authorization"] != "***" || len(accept) != 1 || accept[0] != "application/json" {
		t.Fatalf("headers: %#v", headers)
	}
	payment := record["payment"].(map[string]any)
	if payment["Card_Number"] != "***" || payment["amount"] != "100" {
		t.Fatalf("payment group: %#v", payment)
	}
	body := record["body"].(map[string]any)
	if body["cvv"] != "***" {
		t.Fatalf("body: %#v", body)
	}
	nested := body["nested"].(map[string]any)
	if nested["api_key"] != "***" || nested["ok"] != true {
		t.Fatalf("nested: %#v", nested)
	}
}

func TestErrorMessageStaysWhenCardIsRedacted(t *testing.T) {
	logger, buf := testLogger(t, "payment-api")
	logger.Error("charge failed",
		slog.String("error", "issuer timeout"),
		slog.String("card_number", "4111111111111111"),
	)

	record := decode(t, buf)
	if record["level"] != "ERROR" || record["error"] != "issuer timeout" {
		t.Fatalf("error detail lost: %#v", record)
	}
	if record["card_number"] != "***" || strings.Contains(buf.String(), "4111111111111111") {
		t.Fatalf("card number leaked: %s", buf.String())
	}
}

func TestTraceIDMatchesSpan(t *testing.T) {
	logger, buf := testLogger(t, "payment-api")
	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	ctx, span := tp.Tracer("test").Start(context.Background(), "healthz")
	defer span.End()

	logger.InfoContext(ctx, "healthz")

	record := decode(t, buf)
	want := span.SpanContext().TraceID().String()
	if record["trace_id"] != want {
		t.Fatalf("trace_id = %#v, want %s", record["trace_id"], want)
	}
}

func testLogger(t *testing.T, service string) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	return New(&buf, service, slog.LevelInfo), &buf
}

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("unmarshal %s: %v", buf.String(), err)
	}
	return record
}

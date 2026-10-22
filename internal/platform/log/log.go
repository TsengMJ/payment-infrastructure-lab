// Package log writes JSON logs and redacts sensitive field names at write time.
// Redaction does not depend on the caller remembering to mask a value.
// The message text is not scanned; do not put card numbers or tokens in it,
// and do not log an HTTP request or an upstream response body.
package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/requestid"
)

const redacted = "***"

// secretKeys are matched case-insensitively, including keys nested in groups and maps.
var secretKeys = map[string]struct{}{
	"card_number":   {},
	"authorization": {},
	"pan":           {},
	"cvv":           {},
	"cvc":           {},
	"password":      {},
	"secret":        {},
	"token":         {},
	"api_key":       {},
}

// New returns a JSON logger. Every record includes service, request_id and
// trace_id. request_id comes from the request context and is empty outside a request.
// trace_id is copied from the span on the record's context when that span is valid.
func New(w io.Writer, service string, level slog.Level) *slog.Logger {
	inner := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactAttr,
	})
	return slog.New(traceHandler{next: inner}).With(
		slog.String("service", service),
	)
}

// traceHandler adds trace_id from the active span and request_id from the
// request context. trace_id is the same 32-character hex string Tempo shows.
// A record with no span or no request id gets an empty string.
type traceHandler struct {
	next slog.Handler
}

func (h traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h traceHandler) Handle(ctx context.Context, record slog.Record) error {
	traceID := ""
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		traceID = sc.TraceID().String()
	}
	record.AddAttrs(
		slog.String("trace_id", traceID),
		slog.String("request_id", requestid.FromContext(ctx)),
	)
	return h.next.Handle(ctx, record)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{next: h.next.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{next: h.next.WithGroup(name)}
}

// ParseLevel accepts the LOG_LEVEL values config.Load allows.
func ParseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL: must be one of debug, info, warn, error")
	}
}

func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	if sensitive(attr.Key) {
		attr.Value = slog.StringValue(redacted)
		return attr
	}
	if attr.Value.Kind() == slog.KindAny {
		attr.Value = slog.AnyValue(redactValue(attr.Value.Any()))
	}
	return attr
}

func sensitive(key string) bool {
	_, ok := secretKeys[strings.ToLower(key)]
	return ok
}

// redactValue walks maps with string keys. Other values are returned as-is,
// so an error string stored under a non-sensitive key is kept.
func redactValue(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String {
		return v
	}
	out := make(map[string]any, rv.Len())
	for _, key := range rv.MapKeys() {
		name := key.String()
		val := rv.MapIndex(key).Interface()
		if sensitive(name) {
			out[name] = redacted
			continue
		}
		out[name] = redactValue(val)
	}
	return out
}

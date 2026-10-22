// Package requestid assigns one support id to each HTTP request.
// The id is the merchant-visible Request-Id. It is not the W3C trace id.
package requestid

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"regexp"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const header = "Request-Id"

// accepted is a client-supplied id. Newlines and other characters are rejected
// so the value cannot break a log line or be treated as several headers.
var accepted = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type contextKey struct{}

// Middleware puts a request id on the response, the request context, and the active span.
// A usable Request-Id header is kept. Anything else is replaced.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(header)
		if !accepted.MatchString(id) {
			id = newID()
		}
		w.Header().Set(header, id)
		ctx := context.WithValue(r.Context(), contextKey{}, id)
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			span.SetAttributes(attribute.String("request_id", id))
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the request id, or an empty string when the request has none.
func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}

func newID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "req_unavailable"
	}
	return "req_" + base64.RawURLEncoding.EncodeToString(buf[:])
}

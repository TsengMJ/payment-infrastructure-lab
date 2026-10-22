// Package tenant is the merchant and mode a data query must use.
// The mode comes from the authenticated API key. Callers do not pass a client-supplied mode.
package tenant

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

const (
	// ModeTest is sandboxed traffic.
	ModeTest = "test"
	// ModeLive is live traffic.
	ModeLive = "live"
)

// ErrInvalid is returned when a scope has no merchant or an unknown mode.
var ErrInvalid = errors.New("invalid tenant scope")

type contextKey struct{}

// Scope is the merchant and mode predicate for a tenant-owned query.
// The zero value is invalid. Use New.
type Scope struct {
	merchantID uuid.UUID
	mode       string
	ok         bool
}

// New returns a scope for one merchant and one mode.
// mode is test or live. A nil merchant id is rejected.
func New(merchantID uuid.UUID, mode string) (Scope, error) {
	if merchantID == uuid.Nil || (mode != ModeTest && mode != ModeLive) {
		return Scope{}, ErrInvalid
	}
	return Scope{merchantID: merchantID, mode: mode, ok: true}, nil
}

// Valid reports whether New produced this scope.
func (s Scope) Valid() bool { return s.ok }

// MerchantID returns the merchant primary key. It is nil when Valid is false.
func (s Scope) MerchantID() uuid.UUID { return s.merchantID }

// Mode returns test or live. It is empty when Valid is false.
func (s Scope) Mode() string { return s.mode }

// WithContext stores the scope for handlers and logs on this request.
func WithContext(ctx context.Context, sc Scope) context.Context {
	return context.WithValue(ctx, contextKey{}, sc)
}

// FromContext returns the scope middleware stored for this request.
// An invalid or missing scope reports ok false.
func FromContext(ctx context.Context) (Scope, bool) {
	sc, ok := ctx.Value(contextKey{}).(Scope)
	if !ok || !sc.Valid() {
		return Scope{}, false
	}
	return sc, true
}

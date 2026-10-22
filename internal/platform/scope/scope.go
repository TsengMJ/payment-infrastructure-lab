// Package scope identifies the merchant and mode a transaction query must use.
// The mode comes from the authenticated API key. Callers do not pass a client-supplied mode.
package scope

import (
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
var ErrInvalid = errors.New("invalid scope")

// Scope is the merchant and mode predicate for a transaction query.
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

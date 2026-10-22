// Package auth authenticates bearer tokens and applies the merchant status policy.
// It does not import merchant. The composition root adapts a merchant service to Authenticator.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

const maxBearerTokenLen = 74

var (
	// ErrUnauthenticated means the caller did not present a valid API key.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrForbidden means the key is valid and the merchant may not call the API.
	ErrForbidden = errors.New("forbidden")
	// ErrUnavailable means a dependency required for authentication could not be reached.
	ErrUnavailable = errors.New("auth unavailable")
)

// Identity is the result of a successful key check.
// Status is a fact. CheckMerchantStatus decides whether it may proceed.
type Identity struct {
	MerchantID uuid.UUID
	Status     string
	Mode       string
	KeyID      string
}

// Authenticator verifies a raw API key token.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (Identity, error)
}

type merchantIDKey struct{}

type accessKey struct{}

type access struct {
	mode  string
	keyID string
}

// MerchantID returns the authenticated merchant's primary key.
// The value is a UUID, not the mer_ external form.
func MerchantID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(merchantIDKey{}).(uuid.UUID)
	if !ok || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}

// Mode returns the mode of the API key that authenticated this request.
// It is empty when the request has not passed RequireBearer.
func Mode(ctx context.Context) (string, bool) {
	got, ok := ctx.Value(accessKey{}).(access)
	if !ok || got.mode == "" {
		return "", false
	}
	return got.mode, true
}

// KeyID returns the key id of the API key that authenticated this request.
func KeyID(ctx context.Context) (string, bool) {
	got, ok := ctx.Value(accessKey{}).(access)
	if !ok || got.keyID == "" {
		return "", false
	}
	return got.keyID, true
}

// CheckMerchantStatus allows only an active merchant.
// Every other status, including an unknown value, is rejected.
func CheckMerchantStatus(status string) error {
	if status == "active" {
		return nil
	}
	return ErrForbidden
}

func bearerToken(r *http.Request) (string, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return "", false
	}
	const scheme = "bearer "
	value := values[0]
	if len(value) < len(scheme) || !strings.EqualFold(value[:len(scheme)], scheme) {
		return "", false
	}
	token := value[len(scheme):]
	if token == "" || strings.Contains(token, " ") || len(token) > maxBearerTokenLen {
		return "", false
	}
	return token, true
}

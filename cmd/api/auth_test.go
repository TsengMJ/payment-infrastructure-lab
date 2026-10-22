package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/merchant"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/auth"
)

func TestMerchantAuthenticatorMapsErrors(t *testing.T) {
	id := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b")
	keys := &fakeKeys{principal: merchant.APIKeyPrincipal{
		MerchantID: id,
		KeyID:      "key",
		Mode:       merchant.ModeTest,
		Status:     merchant.StatusActive,
	}}
	authn := newMerchantAuthenticator(keys, merchant.ModeTest)
	got, err := authn.Authenticate(context.Background(), "token")
	if err != nil {
		t.Fatal(err)
	}
	if got.MerchantID != id || got.Status != "active" || got.KeyID != "key" {
		t.Fatalf("identity = %#v", got)
	}

	keys.err = merchant.ErrInvalidAPIKey
	if _, err := authn.Authenticate(context.Background(), "bad"); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("invalid = %v", err)
	}
	keys.err = merchant.ErrUnavailable
	if _, err := authn.Authenticate(context.Background(), "bad"); !errors.Is(err, auth.ErrUnavailable) {
		t.Fatalf("unavailable = %v", err)
	}
	keys.err = errors.New("sql syntax")
	if _, err := authn.Authenticate(context.Background(), "bad"); err == nil || errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("store = %v", err)
	}
}

type fakeKeys struct {
	principal merchant.APIKeyPrincipal
	err       error
}

func (f *fakeKeys) AuthenticateAPIKey(context.Context, string, string) (merchant.APIKeyPrincipal, error) {
	return f.principal, f.err
}

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/merchant"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/auth"
)

type apiKeys interface {
	AuthenticateAPIKey(ctx context.Context, token, processMode string) (merchant.APIKeyPrincipal, error)
}

type merchantAuthenticator struct {
	keys apiKeys
	mode string
}

func newMerchantAuthenticator(keys apiKeys, mode string) auth.Authenticator {
	return merchantAuthenticator{keys: keys, mode: mode}
}

func (a merchantAuthenticator) Authenticate(ctx context.Context, token string) (auth.Identity, error) {
	got, err := a.keys.AuthenticateAPIKey(ctx, token, a.mode)
	switch {
	case err == nil:
		return auth.Identity{
			MerchantID: got.MerchantID,
			Status:     string(got.Status),
			Mode:       got.Mode,
			KeyID:      got.KeyID,
		}, nil
	case errors.Is(err, merchant.ErrInvalidAPIKey):
		return auth.Identity{}, auth.ErrUnauthenticated
	case errors.Is(err, merchant.ErrUnavailable):
		return auth.Identity{}, fmt.Errorf("%w: %w", auth.ErrUnavailable, err)
	default:
		return auth.Identity{}, err
	}
}

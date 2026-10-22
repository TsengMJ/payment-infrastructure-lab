package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/tenant"
)

func TestOwnedAPIKeyRejectsInvalidScope(t *testing.T) {
	var store Store
	_, err := store.GetOwnedAPIKey(context.Background(), tenant.Scope{}, "key")
	if !errors.Is(err, tenant.ErrInvalid) {
		t.Fatalf("get err = %v", err)
	}
	_, err = store.ListOwnedAPIKeys(context.Background(), tenant.Scope{})
	if !errors.Is(err, tenant.ErrInvalid) {
		t.Fatalf("list err = %v", err)
	}
}

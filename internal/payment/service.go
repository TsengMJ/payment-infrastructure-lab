package payment

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/payment/internal/postgres"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/scope"
)

// ErrNotFound is returned when no row matches the scope and id.
// A different merchant or mode is not found. It is not a distinct error.
var ErrNotFound = errors.New("resource not found")

// Service loads mode-scoped payment resources.
type Service struct {
	store *postgres.Store
}

// NewService wires the payment store. A nil pool is a wiring mistake and panics.
func NewService(pool *pgxpool.Pool) *Service {
	if pool == nil {
		panic("payment.NewService: pool is required")
	}
	return &Service{store: postgres.NewStore(pool)}
}

// Get loads one resource for this merchant and mode.
// An invalid scope is a programmer error and is not reported as not found.
func (s *Service) Get(ctx context.Context, sc scope.Scope, id uuid.UUID) (Resource, error) {
	row, err := s.store.Get(ctx, sc, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Resource{}, ErrNotFound
	}
	if err != nil {
		return Resource{}, fmt.Errorf("get resource: %w", err)
	}
	return Resource{
		ID:         row.ID,
		MerchantID: row.MerchantID,
		Mode:       row.Mode,
		CreatedAt:  row.CreatedAt,
	}, nil
}

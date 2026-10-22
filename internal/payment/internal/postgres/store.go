// Package postgres stores mode-scoped payment resources.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/scope"
)

// Row is one payment.resources record.
type Row struct {
	ID         uuid.UUID
	MerchantID uuid.UUID
	Mode       string
	CreatedAt  time.Time
}

// Store reads payment.resources.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore uses pool for every query.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Get loads one row by id, merchant, and mode.
// A missing id, another merchant, and the other mode are all pgx.ErrNoRows.
// There is no query that omits the merchant or the mode.
func (s *Store) Get(ctx context.Context, sc scope.Scope, id uuid.UUID) (Row, error) {
	if !sc.Valid() {
		return Row{}, fmt.Errorf("payment resource: invalid scope")
	}
	const q = `
SELECT id, merchant_id, mode, created_at
FROM payment.resources
WHERE id = $1 AND merchant_id = $2 AND mode = $3`
	var (
		out        Row
		resourceID pgtype.UUID
		merchantID pgtype.UUID
	)
	err := s.pool.QueryRow(ctx, q, pgUUID(id), pgUUID(sc.MerchantID()), sc.Mode()).Scan(
		&resourceID, &merchantID, &out.Mode, &out.CreatedAt,
	)
	if err != nil {
		return Row{}, err
	}
	if !resourceID.Valid || !merchantID.Valid {
		return Row{}, fmt.Errorf("payment resource id is null")
	}
	out.ID = uuid.UUID(resourceID.Bytes)
	out.MerchantID = uuid.UUID(merchantID.Bytes)
	return out, nil
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}
}

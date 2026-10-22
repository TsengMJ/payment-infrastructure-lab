// Package postgres stores merchants in the merchant schema.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewMerchant is a row to insert. Metadata is written by the caller.
type NewMerchant struct {
	ID           uuid.UUID
	Name         string
	Status       string
	ContactEmail *string
	Metadata     []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// MerchantUpdate replaces the mutable columns of one merchant.
type MerchantUpdate struct {
	ID           uuid.UUID
	Name         string
	Status       string
	ContactEmail *string
	Metadata     []byte
	UpdatedAt    time.Time
}

// Row is a merchant loaded from merchant.merchants.
type Row struct {
	ID           uuid.UUID
	Name         string
	Status       string
	ContactEmail *string
	Metadata     []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Store reads and writes merchant.merchants.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore uses pool for every query.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Insert writes one merchant, including metadata, and returns the stored row.
func (s *Store) Insert(ctx context.Context, in NewMerchant) (Row, error) {
	const q = `
INSERT INTO merchant.merchants (id, name, status, contact_email, metadata, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, status, contact_email, metadata, created_at, updated_at`
	row, err := scan(s.pool.QueryRow(ctx, q,
		pgUUID(in.ID), in.Name, in.Status, in.ContactEmail, in.Metadata, in.CreatedAt, in.UpdatedAt))
	if err != nil {
		return Row{}, fmt.Errorf("insert merchant: %w", err)
	}
	return row, nil
}

// Update replaces name, status, contact email, metadata, and updated_at.
// It returns pgx.ErrNoRows when the id is absent.
func (s *Store) Update(ctx context.Context, in MerchantUpdate) (Row, error) {
	const q = `
UPDATE merchant.merchants
SET name = $2, status = $3, contact_email = $4, metadata = $5, updated_at = $6
WHERE id = $1
RETURNING id, name, status, contact_email, metadata, created_at, updated_at`
	row, err := scan(s.pool.QueryRow(ctx, q,
		pgUUID(in.ID), in.Name, in.Status, in.ContactEmail, in.Metadata, in.UpdatedAt))
	if err != nil {
		return Row{}, err
	}
	return row, nil
}

// Get loads one merchant by primary key. It returns pgx.ErrNoRows when the id is absent.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Row, error) {
	const q = `
SELECT id, name, status, contact_email, metadata, created_at, updated_at
FROM merchant.merchants
WHERE id = $1`
	row, err := scan(s.pool.QueryRow(ctx, q, pgUUID(id)))
	if err != nil {
		return Row{}, err
	}
	return row, nil
}

func scan(row pgx.Row) (Row, error) {
	var (
		out   Row
		id    pgtype.UUID
		email *string
		meta  []byte
	)
	err := row.Scan(&id, &out.Name, &out.Status, &email, &meta, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		return Row{}, err
	}
	if !id.Valid {
		return Row{}, fmt.Errorf("merchant id is null")
	}
	out.ID = uuid.UUID(id.Bytes)
	out.ContactEmail = email
	out.Metadata = meta
	return out, nil
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte(id), Valid: true}
}

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/tenant"
)

// NewAPIKey is a row to insert. SecretHash is the SHA-256 of the raw secret.
type NewAPIKey struct {
	KeyID      string
	MerchantID uuid.UUID
	Mode       string
	SecretHash []byte
	Last4      string
	Name       string
	CreatedAt  time.Time
}

// APIKeyRow is a stored key without the secret.
type APIKeyRow struct {
	KeyID      string
	MerchantID uuid.UUID
	Mode       string
	Last4      string
	Name       string
	CreatedAt  time.Time
}

// APIKeyAuth is the row needed to verify a secret and read merchant status.
type APIKeyAuth struct {
	SecretHash []byte
	Mode       string
	MerchantID uuid.UUID
	Status     string
}

// InsertAPIKey inserts a key only when the merchant exists and is active.
// It returns pgx.ErrNoRows when that condition is not met.
func (s *Store) InsertAPIKey(ctx context.Context, in NewAPIKey) (APIKeyRow, error) {
	const q = `
INSERT INTO merchant.api_keys (key_id, merchant_id, mode, secret_hash, last4, name, created_at)
SELECT $1, m.id, $2, $3, $4, $5, $6
FROM merchant.merchants m
WHERE m.id = $7 AND m.status = 'active'
RETURNING key_id, merchant_id, mode, last4, name, created_at`
	var out APIKeyRow
	var merchantID pgtype.UUID
	err := s.pool.QueryRow(ctx, q,
		in.KeyID, in.Mode, in.SecretHash, in.Last4, in.Name, in.CreatedAt, pgUUID(in.MerchantID),
	).Scan(&out.KeyID, &merchantID, &out.Mode, &out.Last4, &out.Name, &out.CreatedAt)
	if err != nil {
		return APIKeyRow{}, err
	}
	if !merchantID.Valid {
		return APIKeyRow{}, fmt.Errorf("merchant id is null")
	}
	out.MerchantID = uuid.UUID(merchantID.Bytes)
	return out, nil
}

// MerchantStatus returns the status of one merchant.
// It returns pgx.ErrNoRows when the id is absent.
func (s *Store) MerchantStatus(ctx context.Context, id uuid.UUID) (string, error) {
	const q = `SELECT status FROM merchant.merchants WHERE id = $1`
	var status string
	err := s.pool.QueryRow(ctx, q, pgUUID(id)).Scan(&status)
	if err != nil {
		return "", err
	}
	return status, nil
}

// GetAPIKey loads a key and its merchant by key_id.
// It returns pgx.ErrNoRows when the key is absent.
func (s *Store) GetAPIKey(ctx context.Context, keyID string) (APIKeyAuth, error) {
	const q = `
SELECT k.secret_hash, k.mode, m.id, m.status
FROM merchant.api_keys k
JOIN merchant.merchants m ON m.id = k.merchant_id
WHERE k.key_id = $1`
	var (
		out        APIKeyAuth
		merchantID pgtype.UUID
	)
	err := s.pool.QueryRow(ctx, q, keyID).Scan(&out.SecretHash, &out.Mode, &merchantID, &out.Status)
	if err != nil {
		return APIKeyAuth{}, err
	}
	if !merchantID.Valid {
		return APIKeyAuth{}, fmt.Errorf("merchant id is null")
	}
	out.MerchantID = uuid.UUID(merchantID.Bytes)
	return out, nil
}

// GetOwnedAPIKey loads one key for this merchant and mode.
// Another merchant, the other mode, and an unknown id are all pgx.ErrNoRows.
// An invalid scope fails before the query.
func (s *Store) GetOwnedAPIKey(ctx context.Context, sc tenant.Scope, keyID string) (APIKeyRow, error) {
	if !sc.Valid() {
		return APIKeyRow{}, fmt.Errorf("api key: %w", tenant.ErrInvalid)
	}
	const q = `
SELECT key_id, merchant_id, mode, last4, name, created_at
FROM merchant.api_keys
WHERE key_id = $1 AND merchant_id = $2 AND mode = $3`
	return scanAPIKey(s.pool.QueryRow(ctx, q, keyID, pgUUID(sc.MerchantID()), sc.Mode()))
}

// ListOwnedAPIKeys returns at most 100 keys for this merchant and mode,
// ordered by created_at then key_id. An invalid scope fails before the query.
func (s *Store) ListOwnedAPIKeys(ctx context.Context, sc tenant.Scope) ([]APIKeyRow, error) {
	if !sc.Valid() {
		return nil, fmt.Errorf("api key: %w", tenant.ErrInvalid)
	}
	const q = `
SELECT key_id, merchant_id, mode, last4, name, created_at
FROM merchant.api_keys
WHERE merchant_id = $1 AND mode = $2
ORDER BY created_at, key_id
LIMIT 100`
	rows, err := s.pool.Query(ctx, q, pgUUID(sc.MerchantID()), sc.Mode())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []APIKeyRow{}
	for rows.Next() {
		row, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scanAPIKey(row interface{ Scan(dest ...any) error }) (APIKeyRow, error) {
	var (
		out        APIKeyRow
		merchantID pgtype.UUID
	)
	err := row.Scan(&out.KeyID, &merchantID, &out.Mode, &out.Last4, &out.Name, &out.CreatedAt)
	if err != nil {
		return APIKeyRow{}, err
	}
	if !merchantID.Valid {
		return APIKeyRow{}, fmt.Errorf("merchant id is null")
	}
	out.MerchantID = uuid.UUID(merchantID.Bytes)
	return out, nil
}

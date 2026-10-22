package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	RevokedAt  *time.Time
}

// RevokedAPIKey is a key whose revoked_at is set.
// Recorded is false when the key was already revoked and no new audit row was written.
type RevokedAPIKey struct {
	MerchantID uuid.UUID
	Mode       string
	RevokedAt  time.Time
	Recorded   bool
}

// CreateAPIKey inserts a key and its audit event in one transaction.
// The insert matches only an active merchant, and a live key also requires live_access.
// It returns pgx.ErrNoRows when that condition is not met.
func (s *Store) CreateAPIKey(ctx context.Context, in NewAPIKey, event AdminEvent) (APIKeyRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return APIKeyRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row, err := insertAPIKey(ctx, tx, in)
	if err != nil {
		return APIKeyRow{}, err
	}
	if err := insertAdminEvent(ctx, tx, event); err != nil {
		return APIKeyRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return APIKeyRow{}, err
	}
	return row, nil
}

func insertAPIKey(ctx context.Context, tx pgx.Tx, in NewAPIKey) (APIKeyRow, error) {
	const q = `
INSERT INTO merchant.api_keys (key_id, merchant_id, mode, secret_hash, last4, name, created_at)
SELECT $1, m.id, $2, $3, $4, $5, $6
FROM merchant.merchants m
WHERE m.id = $7 AND m.status = 'active' AND ($2 <> 'live' OR m.live_access)
RETURNING key_id, merchant_id, mode, last4, name, created_at`
	var out APIKeyRow
	var merchantID pgtype.UUID
	err := tx.QueryRow(ctx, q,
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

// RevokeAPIKey sets revoked_at when it is still null, and writes one audit event.
// A key that is already revoked is returned with Recorded false and no new event.
// It returns pgx.ErrNoRows when the key does not exist.
func (s *Store) RevokeAPIKey(ctx context.Context, keyID string, at time.Time, event AdminEvent) (RevokedAPIKey, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RevokedAPIKey{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const revoke = `
UPDATE merchant.api_keys
SET revoked_at = $2
WHERE key_id = $1 AND revoked_at IS NULL
RETURNING merchant_id, mode, revoked_at`
	var (
		out        RevokedAPIKey
		merchantID pgtype.UUID
	)
	err = tx.QueryRow(ctx, revoke, keyID, at).Scan(&merchantID, &out.Mode, &out.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		existing, err := revokedAPIKey(ctx, tx, keyID)
		if err != nil {
			return RevokedAPIKey{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return RevokedAPIKey{}, err
		}
		return existing, nil
	}
	if err != nil {
		return RevokedAPIKey{}, err
	}
	if !merchantID.Valid {
		return RevokedAPIKey{}, fmt.Errorf("merchant id is null")
	}
	out.MerchantID = uuid.UUID(merchantID.Bytes)
	out.Recorded = true
	event.MerchantID = out.MerchantID
	event.Mode = &out.Mode
	event.KeyID = &keyID
	event.CreatedAt = out.RevokedAt
	if err := insertAdminEvent(ctx, tx, event); err != nil {
		return RevokedAPIKey{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RevokedAPIKey{}, err
	}
	return out, nil
}

func revokedAPIKey(ctx context.Context, tx pgx.Tx, keyID string) (RevokedAPIKey, error) {
	const q = `
SELECT merchant_id, mode, revoked_at
FROM merchant.api_keys
WHERE key_id = $1`
	var (
		out        RevokedAPIKey
		merchantID pgtype.UUID
		revokedAt  *time.Time
	)
	err := tx.QueryRow(ctx, q, keyID).Scan(&merchantID, &out.Mode, &revokedAt)
	if err != nil {
		return RevokedAPIKey{}, err
	}
	if !merchantID.Valid {
		return RevokedAPIKey{}, fmt.Errorf("merchant id is null")
	}
	if revokedAt == nil {
		return RevokedAPIKey{}, fmt.Errorf("api key is active but was not revoked")
	}
	out.MerchantID = uuid.UUID(merchantID.Bytes)
	out.RevokedAt = *revokedAt
	return out, nil
}

// GetAPIKey loads a key and its merchant by key_id.
// It returns pgx.ErrNoRows when the key is absent. Revoked keys are still returned.
func (s *Store) GetAPIKey(ctx context.Context, keyID string) (APIKeyAuth, error) {
	const q = `
SELECT k.secret_hash, k.mode, k.revoked_at, m.id, m.status
FROM merchant.api_keys k
JOIN merchant.merchants m ON m.id = k.merchant_id
WHERE k.key_id = $1`
	var (
		out        APIKeyAuth
		merchantID pgtype.UUID
	)
	err := s.pool.QueryRow(ctx, q, keyID).Scan(&out.SecretHash, &out.Mode, &out.RevokedAt, &merchantID, &out.Status)
	if err != nil {
		return APIKeyAuth{}, err
	}
	if !merchantID.Valid {
		return APIKeyAuth{}, fmt.Errorf("merchant id is null")
	}
	out.MerchantID = uuid.UUID(merchantID.Bytes)
	return out, nil
}

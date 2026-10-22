package merchant

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/merchant/internal/postgres"
)

const (
	tokenPrefixLen = 8
	keyIDBytes     = 16
	secretBytes    = 32
	keyIDLen       = 22
	secretLen      = 43
	tokenLen       = tokenPrefixLen + keyIDLen + 1 + secretLen
)

// CreateAPIKey generates a secret key for an active merchant.
// The returned string is the only copy of the secret. It is not stored.
func (s *Service) CreateAPIKey(ctx context.Context, merchantID ID, name, mode string) (APIKey, string, error) {
	name, err := normalizeKeyName(name)
	if err != nil {
		return APIKey{}, "", err
	}
	token, keyID, secret, err := newToken(mode)
	if err != nil {
		return APIKey{}, "", err
	}
	sum := sha256.Sum256(secret)
	clearBytes(secret)
	row, err := s.store.InsertAPIKey(ctx, postgres.NewAPIKey{
		KeyID:      keyID,
		MerchantID: merchantID.uuid(),
		Mode:       mode,
		SecretHash: sum[:],
		Last4:      token[len(token)-4:],
		Name:       name,
		CreatedAt:  s.clock.Now().UTC(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.missingOrInactive(ctx, merchantID)
		}
		return APIKey{}, "", classifyStoreErr(err)
	}
	return APIKey{
		KeyID:      row.KeyID,
		MerchantID: ID{v: row.MerchantID},
		Mode:       row.Mode,
		Last4:      row.Last4,
		Name:       row.Name,
		CreatedAt:  row.CreatedAt,
	}, token, nil
}

// AuthenticateAPIKey checks the token against processMode.
// A match returns the merchant's current status and does not apply a status policy.
// Malformed tokens, unknown keys, hash mismatches, and mode mismatches are ErrInvalidAPIKey.
func (s *Service) AuthenticateAPIKey(ctx context.Context, token, processMode string) (APIKeyPrincipal, error) {
	mode, keyID, secret, err := parseToken(token)
	if err != nil {
		return APIKeyPrincipal{}, ErrInvalidAPIKey
	}
	defer clearBytes(secret)
	if mode != processMode || (processMode != ModeTest && processMode != ModeLive) {
		return APIKeyPrincipal{}, ErrInvalidAPIKey
	}
	row, err := s.store.GetAPIKey(ctx, keyID)
	sum := sha256.Sum256(secret)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = subtle.ConstantTimeCompare(sum[:], missDigest())
		return APIKeyPrincipal{}, ErrInvalidAPIKey
	}
	if err != nil {
		return APIKeyPrincipal{}, classifyStoreErr(err)
	}
	if len(row.SecretHash) != sha256.Size {
		_ = subtle.ConstantTimeCompare(sum[:], missDigest())
		return APIKeyPrincipal{}, fmt.Errorf("api key hash length")
	}
	var stored [sha256.Size]byte
	copy(stored[:], row.SecretHash)
	match := subtle.ConstantTimeCompare(sum[:], stored[:]) == 1
	if !match || row.Mode != mode {
		return APIKeyPrincipal{}, ErrInvalidAPIKey
	}
	return APIKeyPrincipal{
		MerchantID: row.MerchantID,
		KeyID:      keyID,
		Mode:       row.Mode,
		Status:     Status(row.Status),
	}, nil
}

func (s *Service) missingOrInactive(ctx context.Context, merchantID ID) (APIKey, string, error) {
	status, err := s.store.MerchantStatus(ctx, merchantID.uuid())
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, "", ErrNotFound
	}
	if err != nil {
		return APIKey{}, "", classifyStoreErr(err)
	}
	if status != string(StatusActive) {
		return APIKey{}, "", ErrMerchantInactive
	}
	return APIKey{}, "", fmt.Errorf("create api key: merchant was active but the key was not stored")
}

func newToken(mode string) (token, keyID string, secret []byte, err error) {
	prefix, err := modePrefix(mode)
	if err != nil {
		return "", "", nil, err
	}
	keyRaw := make([]byte, keyIDBytes)
	secret = make([]byte, secretBytes)
	if _, err = rand.Read(keyRaw); err != nil {
		return "", "", nil, fmt.Errorf("api key id: %w", err)
	}
	if _, err = rand.Read(secret); err != nil {
		return "", "", nil, fmt.Errorf("api key secret: %w", err)
	}
	keyID = base64.RawURLEncoding.EncodeToString(keyRaw)
	secretPart := base64.RawURLEncoding.EncodeToString(secret)
	if len(keyID) != keyIDLen || len(secretPart) != secretLen {
		return "", "", nil, fmt.Errorf("api key encoding length")
	}
	return prefix + keyID + "_" + secretPart, keyID, secret, nil
}

func parseToken(token string) (mode, keyID string, secret []byte, err error) {
	if len(token) != tokenLen {
		return "", "", nil, ErrInvalidAPIKey
	}
	mode, err = modeFromPrefix(token[:tokenPrefixLen])
	if err != nil {
		return "", "", nil, ErrInvalidAPIKey
	}
	rest := token[tokenPrefixLen:]
	if rest[keyIDLen] != '_' {
		return "", "", nil, ErrInvalidAPIKey
	}
	keyID = rest[:keyIDLen]
	secretPart := rest[keyIDLen+1:]
	keyRaw, err := base64.RawURLEncoding.DecodeString(keyID)
	if err != nil || len(keyRaw) != keyIDBytes {
		return "", "", nil, ErrInvalidAPIKey
	}
	secret, err = base64.RawURLEncoding.DecodeString(secretPart)
	if err != nil || len(secret) != secretBytes {
		return "", "", nil, ErrInvalidAPIKey
	}
	return mode, keyID, secret, nil
}

func modePrefix(mode string) (string, error) {
	switch mode {
	case ModeTest:
		return "sk_test_", nil
	case ModeLive:
		return "sk_live_", nil
	default:
		return "", ErrInvalidMode
	}
}

func modeFromPrefix(prefix string) (string, error) {
	switch prefix {
	case "sk_test_":
		return ModeTest, nil
	case "sk_live_":
		return ModeLive, nil
	default:
		return "", ErrInvalidAPIKey
	}
}

func normalizeKeyName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > maxKeyNameLen {
		return "", ErrInvalidKeyName
	}
	return name, nil
}

func classifyStoreErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return ErrNotFound
	}
	if unavailable(err) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return fmt.Errorf("merchant store: %w", err)
}

func unavailable(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "08000", "08001", "08003", "08004", "08006", "53300", "57P01", "57P02", "57P03":
			return true
		}
	}
	return false
}

func missDigest() []byte {
	sum := sha256.Sum256([]byte("api-key-lookup-miss"))
	return sum[:]
}

func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

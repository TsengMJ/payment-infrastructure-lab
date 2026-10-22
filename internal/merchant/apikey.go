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

	"github.com/google/uuid"
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
// A live key also requires live access. The returned string is the only copy of the secret.
// The key and its audit event commit together, and the key can be used immediately after that.
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
	now := s.clock.Now().UTC()
	eventID, err := uuid.NewV7()
	if err != nil {
		return APIKey{}, "", fmt.Errorf("api key audit id: %w", err)
	}
	row, err := s.store.CreateAPIKey(ctx, postgres.NewAPIKey{
		KeyID:      keyID,
		MerchantID: merchantID.uuid(),
		Mode:       mode,
		SecretHash: sum[:],
		Last4:      token[len(token)-4:],
		Name:       name,
		CreatedAt:  now,
	}, postgres.AdminEvent{
		EventID:    eventID,
		MerchantID: merchantID.uuid(),
		KeyID:      &keyID,
		Action:     EventAPIKeyCreated,
		Mode:       &mode,
		Actor:      AuditActorManagement,
		CreatedAt:  now,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return s.explainCreateMiss(ctx, merchantID, mode)
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

// RevokeAPIKey records revoked_at. A second revoke returns the original time and does not
// write another audit event. The key id is not a bearer token; a full token is not found.
func (s *Service) RevokeAPIKey(ctx context.Context, keyID string) (RevokedAPIKey, error) {
	if len(keyID) != keyIDLen {
		return RevokedAPIKey{}, ErrNotFound
	}
	eventID, err := uuid.NewV7()
	if err != nil {
		return RevokedAPIKey{}, fmt.Errorf("api key audit id: %w", err)
	}
	row, err := s.store.RevokeAPIKey(ctx, keyID, s.clock.Now().UTC(), postgres.AdminEvent{
		EventID: eventID,
		Action:  EventAPIKeyRevoked,
		Actor:   AuditActorManagement,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return RevokedAPIKey{}, ErrNotFound
	}
	if err != nil {
		return RevokedAPIKey{}, classifyStoreErr(err)
	}
	return RevokedAPIKey{
		KeyID:      keyID,
		MerchantID: ID{v: row.MerchantID},
		Mode:       row.Mode,
		RevokedAt:  row.RevokedAt,
	}, nil
}

// SetLiveAccess grants or removes eligibility to receive new live keys.
// Existing live keys stay valid. The change and its audit event commit together.
func (s *Service) SetLiveAccess(ctx context.Context, merchantID ID, enabled bool) (Merchant, error) {
	now := s.clock.Now().UTC()
	eventID, err := uuid.NewV7()
	if err != nil {
		return Merchant{}, fmt.Errorf("live access audit id: %w", err)
	}
	row, err := s.store.SetLiveAccess(ctx, merchantID.uuid(), enabled, now, postgres.AdminEvent{
		EventID:    eventID,
		MerchantID: merchantID.uuid(),
		Action:     EventLiveAccessSet,
		LiveAccess: &enabled,
		Actor:      AuditActorManagement,
		CreatedAt:  now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Merchant{}, ErrNotFound
	}
	if err != nil {
		return Merchant{}, classifyStoreErr(err)
	}
	return toMerchant(row), nil
}

// AuthenticateAPIKey checks the bearer token.
// A match returns the merchant's current status and does not apply a status policy.
// Malformed tokens, unknown keys, revoked keys, hash mismatches, and prefix/row mode
// mismatches are ErrInvalidAPIKey. The error text does not say which of those it was.
func (s *Service) AuthenticateAPIKey(ctx context.Context, token string) (APIKeyPrincipal, error) {
	mode, keyID, secret, err := parseToken(token)
	if err != nil {
		return APIKeyPrincipal{}, ErrInvalidAPIKey
	}
	defer clearBytes(secret)
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
	if !match || row.Mode != mode || row.RevokedAt != nil {
		return APIKeyPrincipal{}, ErrInvalidAPIKey
	}
	return APIKeyPrincipal{
		MerchantID: row.MerchantID,
		KeyID:      keyID,
		Mode:       row.Mode,
		Status:     Status(row.Status),
	}, nil
}

func (s *Service) explainCreateMiss(ctx context.Context, merchantID ID, mode string) (APIKey, string, error) {
	status, live, err := s.store.MerchantAccess(ctx, merchantID.uuid())
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, "", ErrNotFound
	}
	if err != nil {
		return APIKey{}, "", classifyStoreErr(err)
	}
	if status != string(StatusActive) {
		return APIKey{}, "", ErrMerchantInactive
	}
	if mode == ModeLive && !live {
		return APIKey{}, "", ErrLiveAccessRequired
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

//go:build integration

package merchant

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestCreateAPIKeyStoresHashOnly(t *testing.T) {
	t.Parallel()
	svc, pool, clk := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	merchant, err := svc.Create(ctx, CreateParams{Name: "Northwind"})
	if err != nil {
		t.Fatal(err)
	}
	first, token, err := svc.CreateAPIKey(ctx, merchant.ID, "  primary  ", ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := svc.CreateAPIKey(ctx, merchant.ID, "backup", ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	if first.KeyID == second.KeyID || first.Name != "primary" || second.Name != "backup" {
		t.Fatalf("keys = %#v %#v", first, second)
	}
	if first.MerchantID != merchant.ID || first.Mode != ModeTest || !first.CreatedAt.Equal(clk.Now()) {
		t.Fatalf("key = %#v", first)
	}
	if len(token) != tokenLen || !strings.HasPrefix(token, "sk_test_") || !strings.HasSuffix(token, first.Last4) {
		t.Fatalf("token shape last4=%s", first.Last4)
	}

	var keyID, mode, last4, name string
	var hash []byte
	err = pool.QueryRow(ctx, `
SELECT key_id, mode, last4, name, secret_hash
FROM merchant.api_keys WHERE key_id = $1`, first.KeyID).Scan(&keyID, &mode, &last4, &name, &hash)
	if err != nil {
		t.Fatal(err)
	}
	secretPart := token[len(token)-secretLen:]
	stored := keyID + mode + last4 + name
	if strings.Contains(stored, secretPart) || strings.Contains(stored, token) || bytes.Contains(hash, []byte(token)) {
		t.Fatal("plaintext secret was stored")
	}
	_, _, secret, err := parseToken(token)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(secret)
	if !bytes.Equal(hash, sum[:]) || last4 != token[len(token)-4:] {
		t.Fatal("hash or last4 does not match the secret")
	}

	got, err := svc.AuthenticateAPIKey(ctx, token, ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	if got.MerchantID != merchant.ID.UUID() || got.KeyID != first.KeyID || got.Status != StatusActive || got.Mode != ModeTest {
		t.Fatalf("principal = %#v", got)
	}
}

func TestAuthenticateAPIKeyRejectsWrongSecretAndMode(t *testing.T) {
	t.Parallel()
	svc, pool, _ := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	merchant, err := svc.Create(ctx, CreateParams{Name: "Northwind"})
	if err != nil {
		t.Fatal(err)
	}
	key, token, err := svc.CreateAPIKey(ctx, merchant.ID, "primary", ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	_, _, secret, err := parseToken(token)
	if err != nil {
		t.Fatal(err)
	}
	secret[0] ^= 0xff
	tampered := "sk_test_" + key.KeyID + "_" + base64.RawURLEncoding.EncodeToString(secret)
	if _, err := svc.AuthenticateAPIKey(ctx, tampered, ModeTest); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("tampered err = %v", err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, "not-a-key", ModeTest); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("garbage err = %v", err)
	}

	_, liveToken, err := svc.CreateAPIKey(ctx, merchant.ID, "live", ModeLive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, liveToken, ModeTest); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("cross mode err = %v", err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, liveToken, ModeLive); err != nil {
		t.Fatalf("live auth: %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE merchant.api_keys SET mode = 'live' WHERE key_id = $1`, key.KeyID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, token, ModeTest); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("row mode err = %v", err)
	}
}

func TestCreateAPIKeyRejectsInactiveAndMissingMerchant(t *testing.T) {
	t.Parallel()
	svc, _, _ := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	merchant, err := svc.Create(ctx, CreateParams{Name: "Northwind"})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []Status{StatusSuspended, StatusClosed} {
		if _, err := svc.Update(ctx, merchant.ID, UpdateParams{Name: "Northwind", Status: status}); err != nil {
			t.Fatal(err)
		}
		_, _, err := svc.CreateAPIKey(ctx, merchant.ID, "primary", ModeTest)
		if !errors.Is(err, ErrMerchantInactive) {
			t.Fatalf("status %s err = %v", status, err)
		}
	}
	missing, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.CreateAPIKey(ctx, missing, "primary", ModeTest)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	if _, _, err := svc.CreateAPIKey(ctx, merchant.ID, "  ", ModeTest); !errors.Is(err, ErrInvalidKeyName) {
		t.Fatalf("name err = %v", err)
	}
}

func TestAPIKeyForeignKeyAndStoreOutage(t *testing.T) {
	t.Parallel()
	svc, pool, _ := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	merchant, err := svc.Create(ctx, CreateParams{Name: "Northwind"})
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := svc.CreateAPIKey(ctx, merchant.ID, "primary", ModeTest)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(ctx, `
INSERT INTO merchant.api_keys (key_id, merchant_id, mode, secret_hash, last4, name, created_at)
VALUES ($1, $2, 'test', $3, 'abcd', 'orphan', now())`,
		"orphan-key", uuid.New(), bytes.Repeat([]byte{1}, 32))
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("fk err = %v", err)
	}
	if !errors.Is(classifyStoreErr(pgErr), ErrNotFound) {
		t.Fatal("fk was not mapped to not found")
	}

	expired, cancelExpired := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancelExpired()
	time.Sleep(time.Millisecond)
	_, err = svc.AuthenticateAPIKey(expired, token, ModeTest)
	if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("store outage err = %v", err)
	}
}

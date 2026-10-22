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
	second, secondToken, err := svc.CreateAPIKey(ctx, merchant.ID, "backup", ModeTest)
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

	got, err := svc.AuthenticateAPIKey(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.MerchantID != merchant.ID.UUID() || got.KeyID != first.KeyID || got.Status != StatusActive || got.Mode != ModeTest {
		t.Fatalf("principal = %#v", got)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, secondToken); err != nil {
		t.Fatalf("second key: %v", err)
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
	if _, err := svc.AuthenticateAPIKey(ctx, tampered); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("tampered err = %v", err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, "not-a-key"); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("garbage err = %v", err)
	}

	if _, err := svc.SetLiveAccess(ctx, merchant.ID, true); err != nil {
		t.Fatal(err)
	}
	_, liveToken, err := svc.CreateAPIKey(ctx, merchant.ID, "live", ModeLive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, liveToken); err != nil {
		t.Fatalf("live auth: %v", err)
	}

	mismatch, mismatchID, mismatchSecret, err := newToken(ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(mismatchSecret)
	if _, err := pool.Exec(ctx, `
INSERT INTO merchant.api_keys (key_id, merchant_id, mode, secret_hash, last4, name, created_at)
VALUES ($1, $2, 'live', $3, $4, 'mismatch', now())`,
		mismatchID, merchant.ID.UUID(), sum[:], mismatch[len(mismatch)-4:]); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, mismatch); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("row mode err = %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE merchant.api_keys SET mode = 'live' WHERE key_id = $1`, key.KeyID); err == nil {
		t.Fatal("mode update succeeded")
	}
	if _, err := svc.AuthenticateAPIKey(ctx, token); err != nil {
		t.Fatalf("key still authenticates after rejected mode update: %v", err)
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

func TestRevokeAndLiveAccess(t *testing.T) {
	t.Parallel()
	svc, pool, clk := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	merchant, err := svc.Create(ctx, CreateParams{Name: "Northwind"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.CreateAPIKey(ctx, merchant.ID, "live", ModeLive); !errors.Is(err, ErrLiveAccessRequired) {
		t.Fatalf("live without access err = %v", err)
	}
	clk.Advance(time.Second)
	if _, err := svc.SetLiveAccess(ctx, merchant.ID, true); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Second)
	_, liveToken, err := svc.CreateAPIKey(ctx, merchant.ID, "live", ModeLive)
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Second)
	if _, err := svc.SetLiveAccess(ctx, merchant.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, liveToken); err != nil {
		t.Fatalf("existing live key after access removed: %v", err)
	}
	if _, _, err := svc.CreateAPIKey(ctx, merchant.ID, "live-2", ModeLive); !errors.Is(err, ErrLiveAccessRequired) {
		t.Fatalf("new live key err = %v", err)
	}

	clk.Advance(time.Second)
	first, firstToken, err := svc.CreateAPIKey(ctx, merchant.ID, "primary", ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Second)
	_, secondToken, err := svc.CreateAPIKey(ctx, merchant.ID, "backup", ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, firstToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, secondToken); err != nil {
		t.Fatal(err)
	}

	clk.Advance(time.Minute)
	revoked, err := svc.RevokeAPIKey(ctx, first.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked.RevokedAt.Equal(clk.Now()) || revoked.Mode != ModeTest {
		t.Fatalf("revoked = %+v", revoked)
	}
	revokedErr := authenticateErr(t, svc, ctx, firstToken)
	unknownErr := authenticateErr(t, svc, ctx, "not-a-key")
	if !errors.Is(revokedErr, ErrInvalidAPIKey) || !errors.Is(unknownErr, ErrInvalidAPIKey) {
		t.Fatalf("revoked %v unknown %v", revokedErr, unknownErr)
	}
	if strings.Contains(revokedErr.Error(), "revoked") || strings.Contains(revokedErr.Error(), firstToken) {
		t.Fatalf("error text %q", revokedErr)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, secondToken); err != nil {
		t.Fatalf("backup key: %v", err)
	}
	clk.Advance(time.Minute)
	again, err := svc.RevokeAPIKey(ctx, first.KeyID)
	if err != nil || !again.RevokedAt.Equal(revoked.RevokedAt) {
		t.Fatalf("second revoke = %+v %v", again, err)
	}
	if _, err := svc.RevokeAPIKey(ctx, liveToken); !errors.Is(err, ErrNotFound) {
		t.Fatalf("full token revoke err = %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE merchant.api_keys SET revoked_at = NULL WHERE key_id = $1`, first.KeyID); err == nil {
		t.Fatal("clearing revoked_at succeeded")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM merchant.admin_events WHERE merchant_id = $1`, merchant.ID.UUID()); err == nil {
		t.Fatal("deleting audit events succeeded")
	}

	var actions []string
	rows, err := pool.Query(ctx, `
SELECT action, coalesce(key_id, ''), coalesce(mode, ''), coalesce(live_access::text, ''), actor
FROM merchant.admin_events
WHERE merchant_id = $1
ORDER BY created_at, action`, merchant.ID.UUID())
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var blob strings.Builder
	for rows.Next() {
		var action, keyID, mode, live, actor string
		if err := rows.Scan(&action, &keyID, &mode, &live, &actor); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, action)
		blob.WriteString(action + keyID + mode + live + actor)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		EventLiveAccessSet,
		EventAPIKeyCreated,
		EventLiveAccessSet,
		EventAPIKeyCreated,
		EventAPIKeyCreated,
		EventAPIKeyRevoked,
	}
	if strings.Join(actions, ",") != strings.Join(want, ",") {
		t.Fatalf("actions = %v", actions)
	}
	if strings.Contains(blob.String(), liveToken) || strings.Contains(blob.String(), firstToken) || strings.Contains(blob.String(), secondToken) {
		t.Fatal("audit row contains a secret")
	}
	var revokedEvents int
	if err := pool.QueryRow(ctx, `
SELECT count(*) FROM merchant.admin_events
WHERE key_id = $1 AND action = $2`, first.KeyID, EventAPIKeyRevoked).Scan(&revokedEvents); err != nil {
		t.Fatal(err)
	}
	if revokedEvents != 1 {
		t.Fatalf("revoke events = %d", revokedEvents)
	}
}

func authenticateErr(t *testing.T, svc *Service, ctx context.Context, token string) error {
	t.Helper()
	_, err := svc.AuthenticateAPIKey(ctx, token)
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
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
	_, err = svc.AuthenticateAPIKey(expired, token)
	if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("store outage err = %v", err)
	}
}

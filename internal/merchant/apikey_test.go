package merchant

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNewTokenFormatAndIndependence(t *testing.T) {
	first, firstID, firstSecret, err := newToken(ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	second, secondID, secondSecret, err := newToken(ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != tokenLen || !strings.HasPrefix(first, "sk_test_") {
		t.Fatalf("token = %q", first)
	}
	if firstID == secondID || string(firstSecret) == string(secondSecret) || first == second {
		t.Fatal("two keys shared key id or secret material")
	}
	mode, keyID, secret, err := parseToken(first)
	if err != nil {
		t.Fatal(err)
	}
	if mode != ModeTest || keyID != firstID || string(secret) != string(firstSecret) {
		t.Fatalf("parse = %s %s", mode, keyID)
	}
	if _, _, _, err := newToken("sandbox"); !errors.Is(err, ErrInvalidMode) {
		t.Fatalf("mode err = %v", err)
	}
	live, _, _, err := newToken(ModeLive)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(live, "sk_live_") || len(live) != tokenLen {
		t.Fatalf("live token = %q", live)
	}
}

func TestParseTokenRejectsMalformed(t *testing.T) {
	good, _, _, err := newToken(ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	bad := []string{
		"",
		good[:len(good)-1],
		good + "a",
		"pk_test_" + good[8:],
		good[:30] + "-" + good[31:],
		strings.Replace(good, "sk_test_", "sk_test-", 1),
	}
	for _, token := range bad {
		if _, _, _, err := parseToken(token); !errors.Is(err, ErrInvalidAPIKey) {
			t.Fatalf("parse %q err = %v", token, err)
		}
		if token != "" && strings.Contains(ErrInvalidAPIKey.Error(), token) {
			t.Fatal("invalid key error contains the token")
		}
	}
	raw, err := base64.RawURLEncoding.DecodeString(good[len(good)-secretLen:])
	if err != nil || len(raw) != secretBytes {
		t.Fatalf("secret decode %d %v", len(raw), err)
	}
}

func TestNormalizeKeyName(t *testing.T) {
	got, err := normalizeKeyName("  primary  ")
	if err != nil || got != "primary" {
		t.Fatalf("name = %q, %v", got, err)
	}
	if _, err := normalizeKeyName("  "); !errors.Is(err, ErrInvalidKeyName) {
		t.Fatalf("blank err = %v", err)
	}
	if _, err := normalizeKeyName(strings.Repeat("a", maxKeyNameLen+1)); !errors.Is(err, ErrInvalidKeyName) {
		t.Fatalf("long err = %v", err)
	}
}

func TestClassifyStoreErrSeparatesCauses(t *testing.T) {
	fk := classifyStoreErr(&pgconn.PgError{Code: "23503", Message: "insert into secret sk_test_should_not_leak"})
	if !errors.Is(fk, ErrNotFound) || errors.Is(fk, ErrInvalidAPIKey) {
		t.Fatalf("fk = %v", fk)
	}
	down := classifyStoreErr(&pgconn.PgError{Code: "08006", Message: "connection_failure secret=sk_test_abc"})
	if !errors.Is(down, ErrUnavailable) || errors.Is(down, ErrInvalidAPIKey) {
		t.Fatalf("unavailable = %v", down)
	}
	other := classifyStoreErr(&pgconn.PgError{Code: "23514", Message: "check constraint"})
	if errors.Is(other, ErrInvalidAPIKey) || errors.Is(other, ErrNotFound) || errors.Is(other, ErrUnavailable) {
		t.Fatalf("other = %v", other)
	}
}

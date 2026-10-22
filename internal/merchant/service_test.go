package merchant

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeName(t *testing.T) {
	got, err := normalizeName("  Northwind  ")
	if err != nil {
		t.Fatalf("normalizeName: %v", err)
	}
	if got != "Northwind" {
		t.Fatalf("name = %q", got)
	}
	if _, err := normalizeName("   "); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("blank name err = %v", err)
	}
	if _, err := normalizeName(strings.Repeat("a", maxNameLen)); err != nil {
		t.Fatalf("200 characters: %v", err)
	}
	if _, err := normalizeName(strings.Repeat("a", maxNameLen+1)); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("201 characters err = %v", err)
	}
}

func TestNormalizeEmail(t *testing.T) {
	got, err := normalizeEmail("  Ops@Example.com  ")
	if err != nil {
		t.Fatalf("normalizeEmail: %v", err)
	}
	if got == nil || *got != "Ops@Example.com" {
		t.Fatalf("email = %v", got)
	}
	got, err = normalizeEmail("   ")
	if err != nil {
		t.Fatalf("blank email: %v", err)
	}
	if got != nil {
		t.Fatalf("blank email = %v, want nil", got)
	}
	if _, err := normalizeEmail("not-an-email"); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("missing @ err = %v", err)
	}
	long := strings.Repeat("a", maxEmailLen-2) + "@b"
	if _, err := normalizeEmail(long); err != nil {
		t.Fatalf("254 characters: %v", err)
	}
	if _, err := normalizeEmail(long + "c"); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("255 characters err = %v", err)
	}
}

func TestNormalizeMetadata(t *testing.T) {
	got, err := normalizeMetadata(nil)
	if err != nil || string(got) != "{}" {
		t.Fatalf("nil = %s, %v", got, err)
	}
	got, err = normalizeMetadata(json.RawMessage("  \n  "))
	if err != nil || string(got) != "{}" {
		t.Fatalf("blank = %s, %v", got, err)
	}
	got, err = normalizeMetadata(json.RawMessage(` {"tier":"gold"} `))
	if err != nil || string(got) != `{"tier":"gold"}` {
		t.Fatalf("object = %s, %v", got, err)
	}
	for _, raw := range []string{`[]`, `"x"`, `1`, `null`, `{`, `not-json`} {
		if _, err := normalizeMetadata(json.RawMessage(raw)); !errors.Is(err, ErrInvalidMetadata) {
			t.Fatalf("normalizeMetadata(%s) err = %v", raw, err)
		}
	}
}

func TestNormalizeStatus(t *testing.T) {
	for _, status := range []Status{StatusActive, StatusSuspended, StatusClosed} {
		got, err := normalizeStatus(status)
		if err != nil || got != status {
			t.Fatalf("normalizeStatus(%s) = %s, %v", status, got, err)
		}
	}
	if _, err := normalizeStatus("nope"); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("invalid status err = %v", err)
	}
}

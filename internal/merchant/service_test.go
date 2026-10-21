package merchant

import (
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

package scope

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewRejectsEmptyScope(t *testing.T) {
	if _, err := New(uuid.Nil, ModeTest); !errors.Is(err, ErrInvalid) {
		t.Fatalf("nil merchant err = %v", err)
	}
	id := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b")
	if _, err := New(id, "sandbox"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mode err = %v", err)
	}
	var zero Scope
	if zero.Valid() || zero.MerchantID() != uuid.Nil || zero.Mode() != "" {
		t.Fatal("zero scope is usable")
	}
}

func TestNewKeepsMerchantAndMode(t *testing.T) {
	id := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b")
	got, err := New(id, ModeLive)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Valid() || got.MerchantID() != id || got.Mode() != ModeLive {
		t.Fatalf("scope = %+v", got)
	}
}

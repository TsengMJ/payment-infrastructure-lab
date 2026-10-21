package merchant

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestParseIDRoundTrip(t *testing.T) {
	id, err := NewID()
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	if id.uuid().Version() != 7 {
		t.Fatalf("version = %d, want 7", id.uuid().Version())
	}
	got, err := ParseID(id.String())
	if err != nil {
		t.Fatalf("ParseID: %v", err)
	}
	if got != id {
		t.Fatalf("ParseID(String()) = %s, want %s", got, id)
	}
	if strings.Contains(id.String(), "-") {
		t.Fatalf("external id %s contains a hyphen", id.String())
	}
}

func TestParseIDKnownUUID(t *testing.T) {
	raw := uuid.MustParse("018f6b2e-3c4a-7b2d-8e1f-2a3b4c5d6e7f")
	id := ID{v: raw}
	const want = "mer_018f6b2e3c4a7b2d8e1f2a3b4c5d6e7f"
	if id.String() != want {
		t.Fatalf("String() = %s, want %s", id.String(), want)
	}
	got, err := ParseID(want)
	if err != nil {
		t.Fatalf("ParseID: %v", err)
	}
	if got != id {
		t.Fatalf("parsed = %v, want %v", got, id)
	}
}

func TestParseIDRejects(t *testing.T) {
	valid := "mer_018f6b2e3c4a7b2d8e1f2a3b4c5d6e7f"
	cases := []string{
		"",
		"mer_",
		valid[:len(valid)-1],
		valid + "a",
		"MER_018f6b2e3c4a7b2d8e1f2a3b4c5d6e7f",
		"mer_018F6B2E3C4A7B2D8E1F2A3B4C5D6E7F",
		"mer_018f6b2e-3c4a-7b2d-8e1f-2a3b4c5d6e7f",
		"pay_018f6b2e3c4a7b2d8e1f2a3b4c5d6e7f",
		"018f6b2e-3c4a-7b2d-8e1f-2a3b4c5d6e7f",
		"mer_018f6b2e3c4a7b2d8e1f2a3b4c5d6e7g",
	}
	for _, s := range cases {
		if _, err := ParseID(s); err != ErrInvalidID {
			t.Fatalf("ParseID(%q) err = %v, want ErrInvalidID", s, err)
		}
	}
}

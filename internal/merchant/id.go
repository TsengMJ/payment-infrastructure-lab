package merchant

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const idPrefix = "mer_"

// ID is the merchant primary key. The database stores the UUID.
// String returns the external form, mer_ plus 32 lowercase hex digits.
type ID struct {
	v uuid.UUID
}

// NewID returns a UUIDv7 merchant ID.
func NewID() (ID, error) {
	v, err := uuid.NewV7()
	if err != nil {
		return ID{}, fmt.Errorf("new merchant id: %w", err)
	}
	return ID{v: v}, nil
}

// String returns the external merchant ID.
func (id ID) String() string {
	return idPrefix + hex.EncodeToString(id.v[:])
}

func (id ID) uuid() uuid.UUID { return id.v }

// UUID returns the primary key stored in PostgreSQL.
func (id ID) UUID() uuid.UUID { return id.v }

// ParseID accepts only the external form produced by String.
// It rejects a bare UUID, a wrong prefix, uppercase hex, and hyphens.
func ParseID(s string) (ID, error) {
	rest, ok := strings.CutPrefix(s, idPrefix)
	if !ok || len(rest) != 32 || rest != strings.ToLower(rest) {
		return ID{}, ErrInvalidID
	}
	raw, err := hex.DecodeString(rest)
	if err != nil || len(raw) != 16 {
		return ID{}, ErrInvalidID
	}
	var v uuid.UUID
	copy(v[:], raw)
	return ID{v: v}, nil
}

// Package merchant manages merchants, their API keys and settings.
//
// Other modules may import this package, never merchant/internal.
package merchant

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	maxNameLen  = 200
	maxEmailLen = 254
)

var (
	// ErrNotFound is returned when no merchant has the given ID.
	ErrNotFound = errors.New("merchant not found")
	// ErrInvalidName is returned when the name is empty or longer than 200 characters.
	ErrInvalidName = errors.New("invalid merchant name")
	// ErrInvalidEmail is returned when a contact email is present but longer than 254 characters or has no @.
	ErrInvalidEmail = errors.New("invalid contact email")
	// ErrInvalidID is returned when an external ID is not mer_ plus 32 lowercase hex digits.
	ErrInvalidID = errors.New("invalid merchant id")
	// ErrInvalidMetadata is returned when metadata is not a JSON object.
	ErrInvalidMetadata = errors.New("invalid merchant metadata")
	// ErrInvalidStatus is returned when status is not active, suspended, or closed.
	ErrInvalidStatus = errors.New("invalid merchant status")
)

// Status is the lifecycle of a merchant. The database CHECK allows only these values.
type Status string

const (
	StatusActive    Status = "active"
	StatusSuspended Status = "suspended"
	StatusClosed    Status = "closed"
)

// Merchant is one row of merchant.merchants.
// ContactEmail is empty when the column is NULL.
// Metadata is the stored JSON object. An omitted value is stored as {}.
type Merchant struct {
	ID           ID
	Name         string
	Status       Status
	ContactEmail string
	Metadata     json.RawMessage
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

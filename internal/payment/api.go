// Package payment owns payment intents, attempts and their state machine.
//
// This file is the module's only public surface: other modules may import
// this package, never payment/internal.
//
// Transaction rows carry a mode and are loaded only through a scope, so a
// test key cannot read a live row or another merchant's row.
package payment

import (
	"time"

	"github.com/google/uuid"
)

// Resource is one mode-scoped row. It is not an order.
type Resource struct {
	ID         uuid.UUID
	MerchantID uuid.UUID
	Mode       string
	CreatedAt  time.Time
}

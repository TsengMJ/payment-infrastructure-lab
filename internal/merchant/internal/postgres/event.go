package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AdminEvent is one append-only management action.
// KeyID and Mode are set for key actions. LiveAccess is set for eligibility changes.
type AdminEvent struct {
	EventID    uuid.UUID
	MerchantID uuid.UUID
	KeyID      *string
	Action     string
	Mode       *string
	LiveAccess *bool
	Actor      string
	CreatedAt  time.Time
}

func insertAdminEvent(ctx context.Context, tx pgx.Tx, event AdminEvent) error {
	const q = `
INSERT INTO merchant.admin_events (
  event_id, merchant_id, key_id, action, mode, live_access, actor, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := tx.Exec(ctx, q,
		pgUUID(event.EventID),
		pgUUID(event.MerchantID),
		event.KeyID,
		event.Action,
		event.Mode,
		event.LiveAccess,
		event.Actor,
		event.CreatedAt,
	)
	return err
}

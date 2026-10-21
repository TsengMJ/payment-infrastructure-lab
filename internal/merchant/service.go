package merchant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/merchant/internal/postgres"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/clock"
)

// Service creates and loads merchants. It does not expose HTTP.
type Service struct {
	store *postgres.Store
	clock clock.Clock
}

// NewService wires the merchant store. pool and clk are required;
// a nil argument is a wiring mistake and panics.
func NewService(pool *pgxpool.Pool, clk clock.Clock) *Service {
	if pool == nil || clk == nil {
		panic("merchant.NewService: pool and clock are required")
	}
	return &Service{store: postgres.NewStore(pool), clock: clk}
}

// Create stores a merchant in status active.
// contactEmail is optional; blank means no address is stored.
// metadata is not accepted and stays the database default {}.
func (s *Service) Create(ctx context.Context, name, contactEmail string) (Merchant, error) {
	name, err := normalizeName(name)
	if err != nil {
		return Merchant{}, err
	}
	email, err := normalizeEmail(contactEmail)
	if err != nil {
		return Merchant{}, err
	}
	id, err := NewID()
	if err != nil {
		return Merchant{}, err
	}
	now := s.clock.Now().UTC()
	row, err := s.store.Insert(ctx, postgres.NewMerchant{
		ID:           id.uuid(),
		Name:         name,
		Status:       string(StatusActive),
		ContactEmail: email,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return Merchant{}, fmt.Errorf("create merchant: %w", err)
	}
	return toMerchant(row), nil
}

// Get loads a merchant by its primary key.
func (s *Service) Get(ctx context.Context, id ID) (Merchant, error) {
	row, err := s.store.Get(ctx, id.uuid())
	if errors.Is(err, pgx.ErrNoRows) {
		return Merchant{}, ErrNotFound
	}
	if err != nil {
		return Merchant{}, fmt.Errorf("get merchant: %w", err)
	}
	return toMerchant(row), nil
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	n := utf8.RuneCountInString(name)
	if n < 1 || n > maxNameLen {
		return "", ErrInvalidName
	}
	return name, nil
}

func normalizeEmail(email string) (*string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(email) > maxEmailLen || !strings.Contains(email, "@") {
		return nil, ErrInvalidEmail
	}
	return &email, nil
}

func toMerchant(row postgres.Row) Merchant {
	email := ""
	if row.ContactEmail != nil {
		email = *row.ContactEmail
	}
	meta := json.RawMessage(row.Metadata)
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	return Merchant{
		ID:           ID{v: row.ID},
		Name:         row.Name,
		Status:       Status(row.Status),
		ContactEmail: email,
		Metadata:     meta,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

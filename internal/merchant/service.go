package merchant

import (
	"bytes"
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

// Service creates, updates, and loads merchants. It does not expose HTTP.
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

// CreateParams is the input for a new merchant. Status is always active.
// Metadata is optional; blank means {}.
type CreateParams struct {
	Name         string
	ContactEmail string
	Metadata     json.RawMessage
}

// UpdateParams replaces the mutable fields of an existing merchant.
// Metadata replaces the stored object. It does not merge keys.
type UpdateParams struct {
	Name         string
	ContactEmail string
	Status       Status
	Metadata     json.RawMessage
}

// Create stores a merchant in status active.
// ContactEmail is optional; blank means no address is stored.
func (s *Service) Create(ctx context.Context, in CreateParams) (Merchant, error) {
	name, err := normalizeName(in.Name)
	if err != nil {
		return Merchant{}, err
	}
	email, err := normalizeEmail(in.ContactEmail)
	if err != nil {
		return Merchant{}, err
	}
	meta, err := normalizeMetadata(in.Metadata)
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
		Metadata:     meta,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return Merchant{}, fmt.Errorf("create merchant: %w", err)
	}
	return toMerchant(row), nil
}

// Update replaces name, contact email, status, and metadata.
// created_at stays as it was. updated_at becomes the current clock time.
// A missing merchant returns ErrNotFound.
func (s *Service) Update(ctx context.Context, id ID, in UpdateParams) (Merchant, error) {
	name, err := normalizeName(in.Name)
	if err != nil {
		return Merchant{}, err
	}
	email, err := normalizeEmail(in.ContactEmail)
	if err != nil {
		return Merchant{}, err
	}
	status, err := normalizeStatus(in.Status)
	if err != nil {
		return Merchant{}, err
	}
	meta, err := normalizeMetadata(in.Metadata)
	if err != nil {
		return Merchant{}, err
	}
	row, err := s.store.Update(ctx, postgres.MerchantUpdate{
		ID:           id.uuid(),
		Name:         name,
		Status:       string(status),
		ContactEmail: email,
		Metadata:     meta,
		UpdatedAt:    s.clock.Now().UTC(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Merchant{}, ErrNotFound
	}
	if err != nil {
		return Merchant{}, fmt.Errorf("update merchant: %w", err)
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

func normalizeMetadata(raw json.RawMessage) ([]byte, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return []byte(`{}`), nil
	}
	if !json.Valid(trimmed) || trimmed[0] != '{' {
		return nil, ErrInvalidMetadata
	}
	return trimmed, nil
}

func normalizeStatus(status Status) (Status, error) {
	switch status {
	case StatusActive, StatusSuspended, StatusClosed:
		return status, nil
	default:
		return "", ErrInvalidStatus
	}
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

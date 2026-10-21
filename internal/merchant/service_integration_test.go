//go:build integration

package merchant

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/testutil"
)

var (
	shared     *testutil.Postgres
	externalID = regexp.MustCompile(`^mer_[0-9a-f]{32}$`)
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pg, err := testutil.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres: %v\n", err)
		os.Exit(1)
	}
	shared = pg
	code := m.Run()
	termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer termCancel()
	_ = pg.Close(termCtx)
	os.Exit(code)
}

func TestCreateAndGetWithEmail(t *testing.T) {
	t.Parallel()
	svc, pool, clk := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	got, err := svc.Create(ctx, "  Northwind  ", "  Ops@Example.com  ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Name != "Northwind" {
		t.Fatalf("name = %q", got.Name)
	}
	if got.Status != StatusActive {
		t.Fatalf("status = %s", got.Status)
	}
	if got.ContactEmail != "Ops@Example.com" {
		t.Fatalf("email = %q", got.ContactEmail)
	}
	if !bytes.Equal(got.Metadata, []byte("{}")) {
		t.Fatalf("metadata = %s", got.Metadata)
	}
	if !got.CreatedAt.Equal(clk.Now()) || !got.UpdatedAt.Equal(got.CreatedAt) {
		t.Fatalf("created %s updated %s clock %s", got.CreatedAt, got.UpdatedAt, clk.Now())
	}
	if !externalID.MatchString(got.ID.String()) {
		t.Fatalf("external id %s", got.ID)
	}
	if got.ID.uuid().Version() != 7 {
		t.Fatalf("version = %d", got.ID.uuid().Version())
	}

	var stored string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM merchant.merchants WHERE id = $1::uuid`, got.ID.uuid().String()).Scan(&stored); err != nil {
		t.Fatalf("stored id: %v", err)
	}
	if stored != got.ID.uuid().String() || strings.Contains(stored, "mer_") {
		t.Fatalf("stored id = %s, external = %s", stored, got.ID)
	}

	again, err := svc.Get(ctx, got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if again.ID != got.ID || again.Name != got.Name || again.ContactEmail != got.ContactEmail || again.Status != got.Status {
		t.Fatalf("Get = %+v, want %+v", again, got)
	}
	if !again.CreatedAt.Equal(got.CreatedAt) || !bytes.Equal(again.Metadata, []byte("{}")) {
		t.Fatalf("Get timestamps or metadata = %+v", again)
	}
}

func TestCreateWithoutEmail(t *testing.T) {
	t.Parallel()
	svc, _, _ := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	got, err := svc.Create(ctx, "Acme", "   ")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ContactEmail != "" {
		t.Fatalf("email = %q, want empty", got.ContactEmail)
	}
	if !bytes.Equal(got.Metadata, []byte("{}")) {
		t.Fatalf("metadata = %s", got.Metadata)
	}
}

func TestCreateRejectsInvalidFields(t *testing.T) {
	t.Parallel()
	svc, pool, _ := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := svc.Create(ctx, "   ", "ops@example.com"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("blank name err = %v", err)
	}
	if _, err := svc.Create(ctx, "Acme", "not-an-email"); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("email err = %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM merchant.merchants`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("merchants = %d, want 0", n)
	}
}

func TestGetMissingMerchant(t *testing.T) {
	t.Parallel()
	svc, _, _ := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	missing, err := NewID()
	if err != nil {
		t.Fatalf("NewID: %v", err)
	}
	if _, err := svc.Get(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get err = %v", err)
	}
}

func TestStatusCheckConstraint(t *testing.T) {
	t.Parallel()
	_, pool, _ := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	insert := func(status string) error {
		t.Helper()
		id, err := uuid.NewV7()
		if err != nil {
			t.Fatalf("NewV7: %v", err)
		}
		_, err = pool.Exec(ctx, `
INSERT INTO merchant.merchants (id, name, status, created_at, updated_at)
VALUES ($1::uuid, $2, $3, $4, $4)`, id.String(), "Probe "+status, status, now)
		return err
	}

	err := insert("nope")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("invalid status err = %v", err)
	}
	for _, status := range []string{string(StatusSuspended), string(StatusClosed)} {
		if err := insert(status); err != nil {
			t.Fatalf("insert %s: %v", status, err)
		}
	}
}

func TestMetadataDefaultsWhenOmitted(t *testing.T) {
	t.Parallel()
	_, pool, _ := newFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	id, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("NewV7: %v", err)
	}
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
INSERT INTO merchant.merchants (id, name, status, created_at, updated_at)
VALUES ($1::uuid, 'Default Meta', 'active', $2, $2)`, id.String(), now); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var empty bool
	if err := pool.QueryRow(ctx, `SELECT metadata = '{}'::jsonb FROM merchant.merchants WHERE id = $1::uuid`, id.String()).Scan(&empty); err != nil {
		t.Fatalf("metadata: %v", err)
	}
	if !empty {
		t.Fatal("metadata is not an empty object")
	}
}

func newFixture(t *testing.T) (*Service, *pgxpool.Pool, *testutil.FakeClock) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	dsn := shared.Database(ctx, t)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	clk := testutil.NewFakeClock(time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC))
	return NewService(pool, clk), pool, clk
}

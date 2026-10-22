//go:build integration

package payment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/scope"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/testutil"
)

var sharedDB *testutil.Postgres

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pg, err := testutil.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres: %v\n", err)
		os.Exit(1)
	}
	sharedDB = pg
	code := m.Run()
	termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer termCancel()
	_ = pg.Close(termCtx)
	os.Exit(code)
}

func TestTestKeyCannotReadLiveResource(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := newPool(t, ctx)
	svc := NewService(pool)

	merchantA := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b")
	merchantB := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5c")
	testID := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5d")
	liveID := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5e")
	now := time.Date(2026, 10, 22, 1, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		id       uuid.UUID
		merchant uuid.UUID
		mode     string
	}{
		{testID, merchantA, scope.ModeTest},
		{liveID, merchantA, scope.ModeLive},
	} {
		if _, err := pool.Exec(ctx, `
INSERT INTO payment.resources (id, merchant_id, mode, created_at)
VALUES ($1, $2, $3, $4)`, row.id, row.merchant, row.mode, now); err != nil {
			t.Fatal(err)
		}
	}

	testScope, err := scope.New(merchantA, scope.ModeTest)
	if err != nil {
		t.Fatal(err)
	}
	liveScope, err := scope.New(merchantA, scope.ModeLive)
	if err != nil {
		t.Fatal(err)
	}
	other, err := scope.New(merchantB, scope.ModeTest)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Get(ctx, testScope, liveID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("test key live resource err = %v", err)
	}
	if _, err := svc.Get(ctx, liveScope, testID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("live key test resource err = %v", err)
	}
	if _, err := svc.Get(ctx, other, testID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other merchant err = %v", err)
	}
	got, err := svc.Get(ctx, testScope, testID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != testID || got.MerchantID != merchantA || got.Mode != scope.ModeTest {
		t.Fatalf("resource = %+v", got)
	}
	if _, err := svc.Get(ctx, scope.Scope{}, testID); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("zero scope err = %v", err)
	}
}

func newPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := sharedDB.Database(ctx, t)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

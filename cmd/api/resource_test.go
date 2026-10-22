package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/payment"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/auth"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/scope"
)

func TestResourceHidesOtherModesAndMerchants(t *testing.T) {
	merchantA := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b")
	merchantB := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5c")
	testID := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5d")
	liveID := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5e")
	store := &memResources{rows: []payment.Resource{
		{ID: testID, MerchantID: merchantA, Mode: scope.ModeTest},
		{ID: liveID, MerchantID: merchantA, Mode: scope.ModeLive},
	}}
	token := "sk_test_" + strings.Repeat("a", 22) + "_" + strings.Repeat("b", 43)
	handler := newHandler(fakePinger{}, discardLog(), stubAuth{identity: auth.Identity{
		MerchantID: merchantA,
		Status:     "active",
		Mode:       scope.ModeTest,
		KeyID:      "key-id",
	}}, "", nil, store)

	status, body := getAuth(t, handler, "/v1/resources/"+liveID.String()+"?mode=live", token)
	if status != http.StatusNotFound || !strings.Contains(body, `"not_found"`) || strings.Contains(body, "live") {
		t.Fatalf("live resource via test key: %d %s", status, body)
	}
	if store.got.Mode() != scope.ModeTest || store.got.MerchantID() != merchantA {
		t.Fatalf("scope = merchant %s mode %s", store.got.MerchantID(), store.got.Mode())
	}

	status, body = getAuth(t, handler, "/v1/resources/"+testID.String()+"?mode=live", token)
	if status != http.StatusOK || !strings.Contains(body, `"mode":"test"`) {
		t.Fatalf("test resource: %d %s", status, body)
	}

	other := newHandler(fakePinger{}, discardLog(), stubAuth{identity: auth.Identity{
		MerchantID: merchantB,
		Status:     "active",
		Mode:       scope.ModeTest,
		KeyID:      "other",
	}}, "", nil, store)
	status, _ = getAuth(t, other, "/v1/resources/"+testID.String(), token)
	if status != http.StatusNotFound {
		t.Fatalf("cross merchant status %d", status)
	}

	liveHandler := newHandler(fakePinger{}, discardLog(), stubAuth{identity: auth.Identity{
		MerchantID: merchantA,
		Status:     "active",
		Mode:       scope.ModeLive,
		KeyID:      "live-key",
	}}, "", nil, store)
	liveToken := "sk_live_" + strings.Repeat("c", 22) + "_" + strings.Repeat("d", 43)
	status, _ = getAuth(t, liveHandler, "/v1/resources/"+testID.String(), liveToken)
	if status != http.StatusNotFound {
		t.Fatalf("test resource via live key status %d", status)
	}
	status, body = getAuth(t, liveHandler, "/v1/resources/"+liveID.String(), liveToken)
	if status != http.StatusOK || !strings.Contains(body, `"mode":"live"`) {
		t.Fatalf("live resource: %d %s", status, body)
	}

	status, _ = getAuth(t, handler, "/v1/resources/not-a-uuid", token)
	if status != http.StatusNotFound {
		t.Fatalf("bad id status %d", status)
	}
}

type stubAuth struct {
	identity auth.Identity
}

func (s stubAuth) Authenticate(context.Context, string) (auth.Identity, error) {
	return s.identity, nil
}

type memResources struct {
	rows []payment.Resource
	got  scope.Scope
}

func (m *memResources) Get(_ context.Context, sc scope.Scope, id uuid.UUID) (payment.Resource, error) {
	m.got = sc
	if !sc.Valid() {
		return payment.Resource{}, payment.ErrNotFound
	}
	for _, row := range m.rows {
		if row.ID == id && row.MerchantID == sc.MerchantID() && row.Mode == sc.Mode() {
			return row, nil
		}
	}
	return payment.Resource{}, payment.ErrNotFound
}

func getAuth(t *testing.T, handler http.Handler, path, token string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	return rec.Code, string(body)
}

package main

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/merchant"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/auth"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/tenant"
)

type tenantResource struct {
	name     string
	path     func(owner merchant.APIKey) string
	listPath string
}

func TestCrossTenantResourcesAreHidden(t *testing.T) {
	merchantA := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b")
	merchantB := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5c")
	owned := merchant.APIKey{
		KeyID:      "owned-key-id-22-chars-x",
		MerchantID: mustMerchantID(t, merchantA),
		Mode:       tenant.ModeTest,
		Last4:      "abcd",
		Name:       "primary",
		CreatedAt:  time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC),
	}
	live := owned
	live.KeyID = "live-key-id-22-chars-xx"
	live.Mode = tenant.ModeLive
	other := owned
	other.KeyID = "other-key-id-22-chars-x"
	other.MerchantID = mustMerchantID(t, merchantB)
	store := &memKeys{rows: []merchant.APIKey{owned, live, other}}
	token := bearerFixture()
	handler := newHandler(fakePinger{}, discardLog(), stubAuth{identity: auth.Identity{
		MerchantID: merchantB,
		Status:     "active",
		Mode:       tenant.ModeTest,
		KeyID:      other.KeyID,
	}}, store)

	resources := []tenantResource{
		{
			name:     "api key",
			path:     func(owner merchant.APIKey) string { return "/v1/api_keys/" + owner.KeyID },
			listPath: "/v1/api_keys",
		},
	}
	for _, resource := range resources {
		t.Run(resource.name, func(t *testing.T) {
			status, body := getAuth(t, handler, resource.path(owned)+"?merchant_id="+merchantA.String()+"&mode=live", token)
			missingStatus, missingBody := getAuth(t, handler, resource.path(merchant.APIKey{KeyID: "missing-key-id"})+"?mode=live", token)
			if status != http.StatusNotFound || missingStatus != http.StatusNotFound || body != missingBody {
				t.Fatalf("cross tenant %d %s; missing %d %s", status, body, missingStatus, missingBody)
			}
			if strings.Contains(body, owned.KeyID) || strings.Contains(body, "live") {
				t.Fatalf("body distinguished the resource: %s", body)
			}
			if store.got.MerchantID() != merchantB || store.got.Mode() != tenant.ModeTest {
				t.Fatalf("scope merchant %s mode %s", store.got.MerchantID(), store.got.Mode())
			}
			if resource.listPath == "" {
				return
			}
			listStatus, listBody := getAuth(t, handler, resource.listPath+"?mode=live", token)
			if listStatus != http.StatusOK || strings.Contains(listBody, owned.KeyID) || strings.Contains(listBody, live.KeyID) || !strings.Contains(listBody, other.KeyID) {
				t.Fatalf("list %d %s", listStatus, listBody)
			}
		})
	}
}

func TestOtherModeIsNotFound(t *testing.T) {
	merchantA := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b")
	testKey := merchant.APIKey{
		KeyID:      "test-key-id-22-chars-xx",
		MerchantID: mustMerchantID(t, merchantA),
		Mode:       tenant.ModeTest,
		Last4:      "abcd",
		Name:       "primary",
		CreatedAt:  time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC),
	}
	liveKey := testKey
	liveKey.KeyID = "live-key-id-22-chars-xx"
	liveKey.Mode = tenant.ModeLive
	store := &memKeys{rows: []merchant.APIKey{testKey, liveKey}}
	handler := newHandler(fakePinger{}, discardLog(), stubAuth{identity: auth.Identity{
		MerchantID: merchantA,
		Status:     "active",
		Mode:       tenant.ModeTest,
		KeyID:      testKey.KeyID,
	}}, store)

	status, body := getAuth(t, handler, "/v1/api_keys/"+liveKey.KeyID+"?mode=live", bearerFixture())
	missingStatus, missingBody := getAuth(t, handler, "/v1/api_keys/missing-key-id", bearerFixture())
	if status != http.StatusNotFound || body != missingBody || missingStatus != http.StatusNotFound {
		t.Fatalf("live key %d %s; missing %d %s", status, body, missingStatus, missingBody)
	}
	okStatus, okBody := getAuth(t, handler, "/v1/api_keys/"+testKey.KeyID, bearerFixture())
	if okStatus != http.StatusOK || !strings.Contains(okBody, `"mode":"test"`) || strings.Contains(okBody, "sk_test_") || strings.Contains(okBody, "secret") {
		t.Fatalf("own key %d %s", okStatus, okBody)
	}
}

type stubAuth struct {
	identity auth.Identity
}

func (s stubAuth) Authenticate(context.Context, string) (auth.Identity, error) {
	return s.identity, nil
}

type memKeys struct {
	rows []merchant.APIKey
	got  tenant.Scope
}

func (m *memKeys) GetAPIKey(_ context.Context, sc tenant.Scope, keyID string) (merchant.APIKey, error) {
	m.got = sc
	if !sc.Valid() {
		return merchant.APIKey{}, tenant.ErrInvalid
	}
	for _, row := range m.rows {
		if row.KeyID == keyID && row.MerchantID.UUID() == sc.MerchantID() && row.Mode == sc.Mode() {
			return row, nil
		}
	}
	return merchant.APIKey{}, merchant.ErrNotFound
}

func (m *memKeys) ListAPIKeys(_ context.Context, sc tenant.Scope) ([]merchant.APIKey, error) {
	m.got = sc
	if !sc.Valid() {
		return nil, tenant.ErrInvalid
	}
	out := []merchant.APIKey{}
	for _, row := range m.rows {
		if row.MerchantID.UUID() == sc.MerchantID() && row.Mode == sc.Mode() {
			out = append(out, row)
		}
	}
	return out, nil
}

func mustMerchantID(t *testing.T, id uuid.UUID) merchant.ID {
	t.Helper()
	parsed, err := merchant.ParseID("mer_" + hex.EncodeToString(id[:]))
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func bearerFixture() string {
	return "sk_test_" + strings.Repeat("a", 22) + "_" + strings.Repeat("b", 43)
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

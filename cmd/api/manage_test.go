package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/merchant"
)

const sampleKeyID = "abcdefghijklmnopqrstuv"

func TestManagementRoutesRejectMerchantKeys(t *testing.T) {
	const managementToken = "local-dev-management-token-not-secret"
	admin := &fakeAdmin{}
	handler := newHandler(fakePinger{}, discardLog(), nil, managementToken, admin, nil)
	merchantToken := "sk_test_" + strings.Repeat("a", 22) + "_" + strings.Repeat("b", 43)
	body := `{"name":"primary","mode":"test"}`

	for _, token := range []string{"", merchantToken, "wrong-management-token-value-here"} {
		status, resp := postAuth(t, handler, "/internal/merchants/mer_0123456789abcdef0123456789abcdef/api-keys", token, body)
		if status != http.StatusUnauthorized || !strings.Contains(resp, `"unauthorized"`) || strings.Contains(resp, merchantToken) {
			t.Fatalf("token %q: %d %s", token, status, resp)
		}
	}
	if admin.creates != 0 {
		t.Fatalf("creates = %d", admin.creates)
	}

	status, resp := postAuth(t, handler, "/internal/merchants/mer_0123456789abcdef0123456789abcdef/api-keys", managementToken, body)
	if status != http.StatusCreated || !strings.Contains(resp, `"secret":"sk_test_once"`) {
		t.Fatalf("create: %d %s", status, resp)
	}

	status, resp = postAuth(t, handler, "/internal/merchants/not-a-merchant/api-keys", managementToken, body)
	if status != http.StatusBadRequest || !strings.Contains(resp, `"invalid_request"`) {
		t.Fatalf("bad merchant: %d %s", status, resp)
	}

	admin.createErr = merchant.ErrMerchantInactive
	status, resp = postAuth(t, handler, "/internal/merchants/mer_0123456789abcdef0123456789abcdef/api-keys", managementToken, body)
	if status != http.StatusForbidden || !strings.Contains(resp, `"merchant_inactive"`) {
		t.Fatalf("inactive: %d %s", status, resp)
	}
	admin.createErr = merchant.ErrLiveAccessRequired
	status, resp = postAuth(t, handler, "/internal/merchants/mer_0123456789abcdef0123456789abcdef/api-keys", managementToken, `{"name":"live","mode":"live"}`)
	if status != http.StatusForbidden || !strings.Contains(resp, `"live_access_required"`) {
		t.Fatalf("live access: %d %s", status, resp)
	}
	admin.createErr = merchant.ErrNotFound
	status, _ = postAuth(t, handler, "/internal/merchants/mer_0123456789abcdef0123456789abcdef/api-keys", managementToken, body)
	if status != http.StatusNotFound {
		t.Fatalf("missing merchant status %d", status)
	}

	admin.createErr = nil
	status, resp = postAuth(t, handler, "/internal/merchants/mer_0123456789abcdef0123456789abcdef/live-access", managementToken, `{"enabled":true}`)
	if status != http.StatusOK || !strings.Contains(resp, `"live_access":true`) {
		t.Fatalf("live access set: %d %s", status, resp)
	}
	status, _ = postAuth(t, handler, "/internal/merchants/mer_0123456789abcdef0123456789abcdef/live-access", merchantToken, `{"enabled":false}`)
	if status != http.StatusUnauthorized {
		t.Fatalf("merchant key set live access status %d", status)
	}

	status, resp = postAuth(t, handler, "/internal/api-keys/"+sampleKeyID+"/revoke", managementToken, "")
	if status != http.StatusOK || !strings.Contains(resp, `"key_id":"`+sampleKeyID+`"`) {
		t.Fatalf("revoke: %d %s", status, resp)
	}
	status, _ = postAuth(t, handler, "/internal/api-keys/"+merchantToken+"/revoke", managementToken, "")
	if status != http.StatusNotFound {
		t.Fatalf("token revoke status %d", status)
	}
	status, _ = postAuth(t, handler, "/internal/api-keys/"+sampleKeyID+"/revoke", merchantToken, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("merchant key revoke status %d", status)
	}

	v1 := newHandler(fakePinger{}, discardLog(), newMerchantAuthenticator(&fakeKeys{err: merchant.ErrInvalidAPIKey}), managementToken, admin, nil)
	status, resp = getAuth(t, v1, "/v1/resources/018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b", managementToken)
	if status != http.StatusUnauthorized || !strings.Contains(resp, `"invalid_api_key"`) {
		t.Fatalf("management token on /v1: %d %s", status, resp)
	}
}

type fakeAdmin struct {
	creates   int
	createErr error
	revokes   int
}

func (f *fakeAdmin) CreateAPIKey(context.Context, merchant.ID, string, string) (merchant.APIKey, string, error) {
	f.creates++
	if f.createErr != nil {
		return merchant.APIKey{}, "", f.createErr
	}
	return merchant.APIKey{KeyID: sampleKeyID, Mode: merchant.ModeTest, Last4: "once", Name: "primary"}, "sk_test_once", nil
}

func (f *fakeAdmin) RevokeAPIKey(_ context.Context, keyID string) (merchant.RevokedAPIKey, error) {
	f.revokes++
	if len(keyID) != 22 {
		return merchant.RevokedAPIKey{}, merchant.ErrNotFound
	}
	return merchant.RevokedAPIKey{KeyID: keyID, RevokedAt: time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)}, nil
}

func (f *fakeAdmin) SetLiveAccess(_ context.Context, id merchant.ID, enabled bool) (merchant.Merchant, error) {
	return merchant.Merchant{ID: id, LiveAccess: enabled}, nil
}

func postAuth(t *testing.T, handler http.Handler, path, token, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	return rec.Code, string(resp)
}

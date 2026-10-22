package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/log"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/tenant"
)

func TestBearerMiddleware(t *testing.T) {
	merchantID := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b")
	token := "sk_test_" + strings.Repeat("a", 22) + "_" + strings.Repeat("b", 43)
	if len(token) != maxBearerTokenLen {
		t.Fatalf("fixture len = %d", len(token))
	}

	t.Run("missing", func(t *testing.T) {
		status, body, www := call(t, allow(merchantID), httptest.NewRequest(http.MethodGet, "/v1/orders", nil))
		assertCode(t, status, body, www, http.StatusUnauthorized, "invalid_api_key")
	})

	t.Run("duplicate header", func(t *testing.T) {
		authn := &fakeAuth{identity: Identity{MerchantID: merchantID, Status: "active"}}
		req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
		req.Header.Add("Authorization", "Bearer "+token)
		req.Header.Add("Authorization", "Bearer "+token)
		status, body, www := call(t, authn, req)
		assertCode(t, status, body, www, http.StatusUnauthorized, "invalid_api_key")
		if authn.called {
			t.Fatal("duplicate header reached the authenticator")
		}
	})

	t.Run("malformed", func(t *testing.T) {
		cases := []string{
			"Bearer",
			"Bearer ",
			"Bearer  " + token,
			"Basic " + token,
			"Bearer " + token + " trailing",
			"Bearer " + strings.Repeat("a", maxBearerTokenLen+1),
		}
		for _, header := range cases {
			authn := &fakeAuth{identity: Identity{MerchantID: merchantID, Status: "active"}}
			req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
			req.Header.Set("Authorization", header)
			status, body, www := call(t, authn, req)
			assertCode(t, status, body, www, http.StatusUnauthorized, "invalid_api_key")
			if authn.called {
				t.Fatalf("header %q reached the authenticator", header)
			}
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		authn := &fakeAuth{err: ErrUnauthenticated}
		status, body, www := call(t, authn, bearerRequest(token))
		assertCode(t, status, body, www, http.StatusUnauthorized, "invalid_api_key")
		if !authn.called || authn.got != token {
			t.Fatalf("authenticator token = %q called=%v", authn.got, authn.called)
		}
	})

	t.Run("case insensitive scheme", func(t *testing.T) {
		authn := &fakeAuth{identity: Identity{MerchantID: merchantID, Status: "active", Mode: tenant.ModeTest}}
		req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
		req.Header.Set("Authorization", "bEaReR "+token)
		rec := httptest.NewRecorder()
		require(authn, func(w http.ResponseWriter, r *http.Request) {
			id, ok := MerchantID(r.Context())
			if !ok || id != merchantID || strings.Contains(id.String(), "mer_") {
				t.Fatalf("merchant id = %s ok=%v", id, ok)
			}
			sc, scOK := tenant.FromContext(r.Context())
			if !scOK || sc.MerchantID() != merchantID || sc.Mode() != tenant.ModeTest {
				t.Fatalf("scope ok=%v merchant %s mode %s", scOK, sc.MerchantID(), sc.Mode())
			}
			w.WriteHeader(http.StatusNoContent)
		}).ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status %d", rec.Code)
		}
	})

	for _, status := range []string{"suspended", "closed", "unknown"} {
		t.Run(status, func(t *testing.T) {
			authn := &fakeAuth{identity: Identity{MerchantID: merchantID, Status: status}}
			code, body, www := call(t, authn, bearerRequest(token))
			assertCode(t, code, body, www, http.StatusForbidden, "merchant_inactive")
			if www != "" {
				t.Fatalf("WWW-Authenticate = %q", www)
			}
		})
	}

	t.Run("store down", func(t *testing.T) {
		authn := &fakeAuth{err: errors.Join(ErrUnavailable, errors.New("dial tcp secret=sk_test_should_not_leak"))}
		status, body, _ := call(t, authn, bearerRequest(token))
		assertCode(t, status, body, "", http.StatusServiceUnavailable, "internal_error")
		if strings.Contains(string(body), "sk_test_") || strings.Contains(string(body), "dial") {
			t.Fatalf("body leaked internals: %s", body)
		}
	})

	t.Run("identity without mode", func(t *testing.T) {
		authn := &fakeAuth{identity: Identity{MerchantID: merchantID, Status: "active"}}
		called := false
		rec := httptest.NewRecorder()
		require(authn, func(http.ResponseWriter, *http.Request) {
			called = true
		}).ServeHTTP(rec, bearerRequest(token))
		if called {
			t.Fatal("handler ran without a scope")
		}
		assertCode(t, rec.Code, rec.Body.Bytes(), rec.Header().Get("WWW-Authenticate"), http.StatusInternalServerError, "internal_error")
	})

	t.Run("unexpected store error", func(t *testing.T) {
		authn := &fakeAuth{err: errors.New("SQLSTATE 42601 syntax error near secret")}
		status, body, www := call(t, authn, bearerRequest(token))
		assertCode(t, status, body, www, http.StatusInternalServerError, "internal_error")
		if strings.Contains(string(body), "SQLSTATE") || strings.Contains(string(body), "secret") {
			t.Fatalf("body leaked internals: %s", body)
		}
	})
}

func TestMiddlewareDoesNotLogTheToken(t *testing.T) {
	token := "sk_test_" + strings.Repeat("a", 22) + "_" + strings.Repeat("b", 43)
	var buf bytes.Buffer
	logger := log.New(&buf, "payment-api", slog.LevelDebug)
	authn := &fakeAuth{err: ErrUnauthenticated}
	handler := RequireBearer(authn)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := bearerRequest(token)
	handler.ServeHTTP(httptest.NewRecorder(), req)
	logger.InfoContext(req.Context(), "request", slog.String("authorization", req.Header.Get("Authorization")))
	if strings.Contains(buf.String(), token) {
		t.Fatalf("log contains the api key: %s", buf.String())
	}
}

func TestCheckMerchantStatus(t *testing.T) {
	if err := CheckMerchantStatus("active"); err != nil {
		t.Fatal(err)
	}
	if err := CheckMerchantStatus("suspended"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v", err)
	}
}

type fakeAuth struct {
	identity Identity
	err      error
	called   bool
	got      string
}

func (f *fakeAuth) Authenticate(_ context.Context, token string) (Identity, error) {
	f.called = true
	f.got = token
	return f.identity, f.err
}

func allow(id uuid.UUID) *fakeAuth {
	return &fakeAuth{identity: Identity{MerchantID: id, Status: "active", Mode: tenant.ModeTest}}
}

func require(authn Authenticator, next http.HandlerFunc) http.Handler {
	return RequireBearer(authn)(next)
}

func bearerRequest(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/v1/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func call(t *testing.T, authn Authenticator, req *http.Request) (int, []byte, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	require(authn, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := MerchantID(r.Context()); !ok {
			t.Fatal("missing merchant id")
		}
		w.WriteHeader(http.StatusNoContent)
	}).ServeHTTP(rec, req)
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	return rec.Code, body, rec.Header().Get("WWW-Authenticate")
}

func assertCode(t *testing.T, status int, body []byte, www string, wantStatus int, wantCode string) {
	t.Helper()
	if status != wantStatus {
		t.Fatalf("status %d, want %d, body %s", status, wantStatus, body)
	}
	var got errorBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != wantCode {
		t.Fatalf("code %q, want %q", got.Code, wantCode)
	}
	if wantStatus == http.StatusUnauthorized && www != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", www)
	}
	if wantStatus != http.StatusUnauthorized && www != "" {
		t.Fatalf("WWW-Authenticate = %q", www)
	}
}

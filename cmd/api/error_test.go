package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/auth"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/tenant"
)

func TestErrorResponsesShareOneShape(t *testing.T) {
	merchantID := uuid.MustParse("018f1a2b-3c4d-7e5f-8a9b-0c1d2e3f4a5b")
	token := bearerFixture()
	okAuth := stubAuth{identity: auth.Identity{
		MerchantID: merchantID,
		Status:     "active",
		Mode:       tenant.ModeTest,
		KeyID:      strings.Repeat("a", 22),
	}}
	store := &memKeys{}

	cases := []struct {
		name      string
		handler   http.Handler
		path      string
		auth      bool
		status    int
		code      string
		retryable bool
		param     string
	}{
		{
			name:    "validation",
			handler: newHandler(fakePinger{}, discardLog(), okAuth, store),
			path:    "/v1/api_keys/not-a-key",
			auth:    true,
			status:  http.StatusBadRequest,
			code:    "parameter_invalid",
			param:   "key_id",
		},
		{
			name:    "authentication",
			handler: newHandler(fakePinger{}, discardLog(), okAuth, store),
			path:    "/v1/api_keys/" + strings.Repeat("d", 22),
			status:  http.StatusUnauthorized,
			code:    "invalid_api_key",
		},
		{
			name:    "missing",
			handler: newHandler(fakePinger{}, discardLog(), okAuth, store),
			path:    "/v1/api_keys/" + strings.Repeat("d", 22),
			auth:    true,
			status:  http.StatusNotFound,
			code:    "resource_missing",
		},
		{
			name: "internal",
			handler: newHandler(fakePinger{}, discardLog(), stubAuth{
				err: errors.New("SQLSTATE 42601 syntax error near secret sk_test_should_not_leak"),
			}, store),
			path:      "/v1/api_keys/" + strings.Repeat("d", 22),
			auth:      true,
			status:    http.StatusInternalServerError,
			code:      "internal_error",
			retryable: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Header.Set("Request-Id", "err-shape")
			if tc.auth {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			rec := httptest.NewRecorder()
			tc.handler.ServeHTTP(rec, req)
			body, err := io.ReadAll(rec.Body)
			if err != nil {
				t.Fatal(err)
			}
			if rec.Code != tc.status {
				t.Fatalf("status %d body %s", rec.Code, body)
			}
			if strings.Contains(string(body), "SQLSTATE") || strings.Contains(string(body), "sk_test_") {
				t.Fatalf("body leaked internals: %s", body)
			}
			var got struct {
				Error struct {
					Type      string `json:"type"`
					Code      string `json:"code"`
					Message   string `json:"message"`
					Param     string `json:"param"`
					RequestID string `json:"request_id"`
					Retryable bool   `json:"retryable"`
				} `json:"error"`
			}
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if got.Error.Type == "" || got.Error.Code != tc.code || got.Error.Message == "" || got.Error.RequestID != "err-shape" || got.Error.Retryable != tc.retryable || got.Error.Param != tc.param {
				t.Fatalf("error = %#v", got.Error)
			}
		})
	}
}

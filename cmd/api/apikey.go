package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/merchant"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/tenant"
)

type apiKeyReader interface {
	GetAPIKey(ctx context.Context, sc tenant.Scope, keyID string) (merchant.APIKey, error)
	ListAPIKeys(ctx context.Context, sc tenant.Scope) ([]merchant.APIKey, error)
}

type apiKeyResponse struct {
	KeyID     string    `json:"key_id"`
	Mode      string    `json:"mode"`
	Last4     string    `json:"last4"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type apiKeyListResponse struct {
	Data []apiKeyResponse `json:"data"`
}

func listAPIKeys(keys apiKeyReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sc, ok := scopeOrError(w, keys, r)
		if !ok {
			return
		}
		rows, err := keys.ListAPIKeys(r.Context(), sc)
		if err != nil {
			writeCode(w, http.StatusInternalServerError, "internal_error")
			return
		}
		out := apiKeyListResponse{Data: make([]apiKeyResponse, 0, len(rows))}
		for _, row := range rows {
			out.Data = append(out.Data, toAPIKeyResponse(row))
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getAPIKey(keys apiKeyReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sc, ok := scopeOrError(w, keys, r)
		if !ok {
			return
		}
		got, err := keys.GetAPIKey(r.Context(), sc, chi.URLParam(r, "key_id"))
		if errors.Is(err, merchant.ErrNotFound) {
			writeCode(w, http.StatusNotFound, "not_found")
			return
		}
		if err != nil {
			writeCode(w, http.StatusInternalServerError, "internal_error")
			return
		}
		writeJSON(w, http.StatusOK, toAPIKeyResponse(got))
	}
}

func scopeOrError(w http.ResponseWriter, keys apiKeyReader, r *http.Request) (tenant.Scope, bool) {
	if keys == nil {
		writeCode(w, http.StatusInternalServerError, "internal_error")
		return tenant.Scope{}, false
	}
	sc, ok := tenant.FromContext(r.Context())
	if !ok {
		writeCode(w, http.StatusInternalServerError, "internal_error")
		return tenant.Scope{}, false
	}
	return sc, true
}

func toAPIKeyResponse(row merchant.APIKey) apiKeyResponse {
	return apiKeyResponse{
		KeyID:     row.KeyID,
		Mode:      row.Mode,
		Last4:     row.Last4,
		Name:      row.Name,
		CreatedAt: row.CreatedAt,
	}
}

func writeCode(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, struct {
		Code string `json:"code"`
	}{Code: code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"code":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

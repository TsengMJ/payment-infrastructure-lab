package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/merchant"
)

type keyAdmin interface {
	CreateAPIKey(ctx context.Context, merchantID merchant.ID, name, mode string) (merchant.APIKey, string, error)
	RevokeAPIKey(ctx context.Context, keyID string) (merchant.RevokedAPIKey, error)
	SetLiveAccess(ctx context.Context, merchantID merchant.ID, enabled bool) (merchant.Merchant, error)
}

func requireManagement(configured string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := managementBearer(r)
			if !ok || !managementTokenMatches(token, configured) {
				writeCode(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func managementBearer(r *http.Request) (string, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return "", false
	}
	const scheme = "bearer "
	value := values[0]
	if len(value) < len(scheme) || !strings.EqualFold(value[:len(scheme)], scheme) {
		return "", false
	}
	token := value[len(scheme):]
	if token == "" || strings.Contains(token, " ") || len(token) > 256 {
		return "", false
	}
	return token, true
}

func managementTokenMatches(presented, configured string) bool {
	if presented == "" || configured == "" {
		return false
	}
	sumPresented := sha256.Sum256([]byte(presented))
	sumConfigured := sha256.Sum256([]byte(configured))
	return subtle.ConstantTimeCompare(sumPresented[:], sumConfigured[:]) == 1
}

type createKeyBody struct {
	Name string `json:"name"`
	Mode string `json:"mode"`
}

type createKeyResponse struct {
	KeyID  string `json:"key_id"`
	Mode   string `json:"mode"`
	Last4  string `json:"last4"`
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

func createAPIKey(keys keyAdmin) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if keys == nil {
			writeCode(w, http.StatusInternalServerError, "internal_error")
			return
		}
		merchantID, ok := pathMerchantID(w, r)
		if !ok {
			return
		}
		var body createKeyBody
		if !readJSON(w, r, &body) {
			return
		}
		key, secret, err := keys.CreateAPIKey(r.Context(), merchantID, body.Name, body.Mode)
		if err != nil {
			writeAdminError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, createKeyResponse{
			KeyID:  key.KeyID,
			Mode:   key.Mode,
			Last4:  key.Last4,
			Name:   key.Name,
			Secret: secret,
		})
	}
}

type liveAccessBody struct {
	Enabled *bool `json:"enabled"`
}

type liveAccessResponse struct {
	ID         string `json:"id"`
	LiveAccess bool   `json:"live_access"`
}

func setLiveAccess(keys keyAdmin) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if keys == nil {
			writeCode(w, http.StatusInternalServerError, "internal_error")
			return
		}
		merchantID, ok := pathMerchantID(w, r)
		if !ok {
			return
		}
		var body liveAccessBody
		if !readJSON(w, r, &body) {
			return
		}
		if body.Enabled == nil {
			writeCode(w, http.StatusBadRequest, "invalid_request")
			return
		}
		got, err := keys.SetLiveAccess(r.Context(), merchantID, *body.Enabled)
		if err != nil {
			writeAdminError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, liveAccessResponse{ID: got.ID.String(), LiveAccess: got.LiveAccess})
	}
}

type revokeResponse struct {
	KeyID     string    `json:"key_id"`
	RevokedAt time.Time `json:"revoked_at"`
}

func revokeAPIKey(keys keyAdmin) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if keys == nil {
			writeCode(w, http.StatusInternalServerError, "internal_error")
			return
		}
		revoked, err := keys.RevokeAPIKey(r.Context(), chi.URLParam(r, "key_id"))
		if err != nil {
			writeAdminError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, revokeResponse{KeyID: revoked.KeyID, RevokedAt: revoked.RevokedAt})
	}
}

func pathMerchantID(w http.ResponseWriter, r *http.Request) (merchant.ID, bool) {
	id, err := merchant.ParseID(chi.URLParam(r, "merchant_id"))
	if err != nil {
		writeCode(w, http.StatusBadRequest, "invalid_request")
		return merchant.ID{}, false
	}
	return id, true
}

func writeAdminError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, merchant.ErrNotFound):
		writeCode(w, http.StatusNotFound, "not_found")
	case errors.Is(err, merchant.ErrMerchantInactive):
		writeCode(w, http.StatusForbidden, "merchant_inactive")
	case errors.Is(err, merchant.ErrLiveAccessRequired):
		writeCode(w, http.StatusForbidden, "live_access_required")
	case errors.Is(err, merchant.ErrInvalidKeyName), errors.Is(err, merchant.ErrInvalidMode), errors.Is(err, merchant.ErrInvalidID):
		writeCode(w, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, merchant.ErrUnavailable):
		writeCode(w, http.StatusServiceUnavailable, "internal_error")
	default:
		writeCode(w, http.StatusInternalServerError, "internal_error")
	}
}

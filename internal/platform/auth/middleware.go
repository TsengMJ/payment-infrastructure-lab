package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type errorBody struct {
	Code string `json:"code"`
}

// RequireBearer requires one Authorization: Bearer token on the wrapped handler.
// Invalid keys are 401, inactive merchants are 403, and store failures are 503 or 500.
// Response bodies contain only a code.
func RequireBearer(authenticator Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeCode(w, http.StatusUnauthorized, "invalid_api_key")
				return
			}
			if authenticator == nil {
				writeCode(w, http.StatusInternalServerError, "internal_error")
				return
			}
			identity, err := authenticator.Authenticate(r.Context(), token)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			if err := CheckMerchantStatus(identity.Status); err != nil {
				writeCode(w, http.StatusForbidden, "merchant_inactive")
				return
			}
			ctx := contextWithMerchant(r.Context(), identity.MerchantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func contextWithMerchant(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, merchantIDKey{}, id)
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		writeCode(w, http.StatusUnauthorized, "invalid_api_key")
	case errors.Is(err, ErrUnavailable):
		writeCode(w, http.StatusServiceUnavailable, "internal_error")
	default:
		writeCode(w, http.StatusInternalServerError, "internal_error")
	}
}

func writeCode(w http.ResponseWriter, status int, code string) {
	body, err := json.Marshal(errorBody{Code: code})
	if err != nil {
		http.Error(w, `{"code":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

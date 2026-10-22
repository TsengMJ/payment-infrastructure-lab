package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/apierror"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/tenant"
)

// RequireBearer requires one Authorization: Bearer token on the wrapped handler.
// Invalid keys are 401, inactive merchants are 403, and store failures are 503 or 500.
// Response bodies use the shared API error object and do not include the cause.
func RequireBearer(authenticator Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				apierror.Write(w, r, apierror.InvalidAPIKey, "")
				return
			}
			if authenticator == nil {
				apierror.Write(w, r, apierror.Internal, "")
				return
			}
			identity, err := authenticator.Authenticate(r.Context(), token)
			if err != nil {
				writeAuthError(w, r, err)
				return
			}
			if err := CheckMerchantStatus(identity.Status); err != nil {
				apierror.Write(w, r, apierror.MerchantInactive, "")
				return
			}
			sc, err := tenant.New(identity.MerchantID, identity.Mode)
			if err != nil {
				apierror.Write(w, r, apierror.Internal, "")
				return
			}
			ctx := tenant.WithContext(contextWithMerchant(r.Context(), identity.MerchantID), sc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func contextWithMerchant(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, merchantIDKey{}, id)
}

func writeAuthError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		apierror.Write(w, r, apierror.InvalidAPIKey, "")
	case errors.Is(err, ErrUnavailable):
		apierror.Write(w, r, apierror.ServiceUnavailable, "")
	default:
		apierror.Write(w, r, apierror.Internal, "")
	}
}

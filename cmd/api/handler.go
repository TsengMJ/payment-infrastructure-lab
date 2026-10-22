package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/auth"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/otel"
)

type pinger interface {
	Ping(ctx context.Context) error
}

func newHandler(ready pinger, logger *slog.Logger, authenticator auth.Authenticator, keys apiKeyReader) http.Handler {
	r := chi.NewRouter()
	r.Use(otel.HTTPMiddleware("payment-api"))
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if logger != nil {
			logger.InfoContext(r.Context(), "request",
				slog.String("http.method", r.Method),
				slog.String("http.route", "/healthz"),
			)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := ready.Ping(ctx); err != nil {
			http.Error(w, "database unavailable\n", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	r.Route("/v1", func(r chi.Router) {
		r.Use(auth.RequireBearer(authenticator))
		r.Get("/api_keys", listAPIKeys(keys))
		r.Get("/api_keys/{key_id}", getAPIKey(keys))
	})
	return r
}

package main

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/payment"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/auth"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/scope"
)

type resourceResponse struct {
	ID   string `json:"id"`
	Mode string `json:"mode"`
}

func getResource(resources resourceReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if resources == nil {
			writeCode(w, http.StatusInternalServerError, "internal_error")
			return
		}
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeCode(w, http.StatusNotFound, "not_found")
			return
		}
		merchantID, ok := auth.MerchantID(r.Context())
		mode, modeOK := auth.Mode(r.Context())
		if !ok || !modeOK {
			writeCode(w, http.StatusInternalServerError, "internal_error")
			return
		}
		sc, err := scope.New(merchantID, mode)
		if err != nil {
			writeCode(w, http.StatusInternalServerError, "internal_error")
			return
		}
		got, err := resources.Get(r.Context(), sc, id)
		if errors.Is(err, payment.ErrNotFound) {
			writeCode(w, http.StatusNotFound, "not_found")
			return
		}
		if err != nil {
			writeCode(w, http.StatusInternalServerError, "internal_error")
			return
		}
		writeJSON(w, http.StatusOK, resourceResponse{ID: got.ID.String(), Mode: got.Mode})
	}
}

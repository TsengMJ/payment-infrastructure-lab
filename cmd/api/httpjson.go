package main

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Code string `json:"code"`
}

func writeCode(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, errorBody{Code: code})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	encoded, err := json.Marshal(body)
	if err != nil {
		http.Error(w, `{"code":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(encoded)
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeCode(w, http.StatusBadRequest, "invalid_request")
		return false
	}
	return true
}

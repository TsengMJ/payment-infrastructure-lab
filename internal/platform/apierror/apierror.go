// Package apierror writes the merchant-facing error body.
// Callers pass a code. The Go error text is never copied into the response.
package apierror

import (
	"encoding/json"
	"net/http"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/requestid"
)

// Code is a stable merchant-facing error code.
type Code string

const (
	// ParameterInvalid means the request is malformed. Retrying the same request will fail.
	ParameterInvalid Code = "parameter_invalid"
	// InvalidAPIKey means the caller did not present a usable API key.
	InvalidAPIKey Code = "invalid_api_key"
	// MerchantInactive means the key is valid and the merchant may not call the API.
	MerchantInactive Code = "merchant_inactive"
	// ResourceMissing means the id is absent from this merchant and mode.
	// Another merchant's resource uses the same code.
	ResourceMissing Code = "resource_missing"
	// Internal means the server failed unexpectedly.
	Internal Code = "internal_error"
	// ServiceUnavailable means a dependency could not be reached.
	ServiceUnavailable Code = "service_unavailable"
)

type spec struct {
	status    int
	typ       string
	message   string
	retryable bool
}

var catalog = map[Code]spec{
	ParameterInvalid:   {http.StatusBadRequest, "invalid_request_error", "A parameter is invalid.", false},
	InvalidAPIKey:      {http.StatusUnauthorized, "authentication_error", "The API key is invalid.", false},
	MerchantInactive:   {http.StatusForbidden, "invalid_request_error", "The merchant cannot call the API.", false},
	ResourceMissing:    {http.StatusNotFound, "invalid_request_error", "No such resource.", false},
	Internal:           {http.StatusInternalServerError, "api_error", "An internal error occurred.", true},
	ServiceUnavailable: {http.StatusServiceUnavailable, "api_error", "The service is temporarily unavailable.", true},
}

// Codes returns every code the catalog can write, in a stable order.
func Codes() []Code {
	return []Code{
		ParameterInvalid,
		InvalidAPIKey,
		MerchantInactive,
		ResourceMissing,
		Internal,
		ServiceUnavailable,
	}
}

type body struct {
	Error detail `json:"error"`
}

type detail struct {
	Type      string `json:"type"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Param     string `json:"param,omitempty"`
	RequestID string `json:"request_id"`
	Retryable bool   `json:"retryable"`
}

// Write sends the catalog entry for code. An unknown code is written as an internal error.
// param is omitted when empty. retryable means the same request may be tried again;
// it does not mean a payment failed.
func Write(w http.ResponseWriter, r *http.Request, code Code, param string) {
	item, ok := catalog[code]
	if !ok {
		item = catalog[Internal]
		code = Internal
		param = ""
	}
	payload, err := json.Marshal(body{Error: detail{
		Type:      item.typ,
		Code:      string(code),
		Message:   item.message,
		Param:     param,
		RequestID: requestid.FromContext(r.Context()),
		Retryable: item.retryable,
	}})
	if err != nil {
		http.Error(w, `{"error":{"type":"api_error","code":"internal_error","message":"An internal error occurred.","request_id":"","retryable":true}}`, http.StatusInternalServerError)
		return
	}
	if item.status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(item.status)
	_, _ = w.Write(payload)
}

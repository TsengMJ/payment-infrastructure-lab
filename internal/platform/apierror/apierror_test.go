package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteUsesOneShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/api_keys/x", nil)
	rec := httptest.NewRecorder()
	Write(rec, req, ParameterInvalid, "key_id")

	if rec.Code != http.StatusBadRequest || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("status %d type %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	var got body
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Type != "invalid_request_error" || got.Error.Code != "parameter_invalid" || got.Error.Param != "key_id" || got.Error.Retryable {
		t.Fatalf("error = %#v", got.Error)
	}
	if got.Error.Message == "" || strings.Contains(got.Error.Message, "sql") {
		t.Fatalf("message = %q", got.Error.Message)
	}
}

func TestInternalErrorHidesTheCause(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	Write(rec, req, Code("SQLSTATE 42601 syntax error near secret"), "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
	text := rec.Body.String()
	if strings.Contains(text, "SQLSTATE") || strings.Contains(text, "secret") || !strings.Contains(text, `"retryable":true`) {
		t.Fatalf("body = %s", text)
	}
}

func TestCatalogListsEveryCode(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "docs", "api", "errors.md")
	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(doc)
	for _, code := range Codes() {
		if !strings.Contains(text, string(code)) {
			t.Fatalf("docs/api/errors.md missing %s", code)
		}
	}
}

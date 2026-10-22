package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsAllowsOnlyInternalCallers(t *testing.T) {
	metrics := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := newMetricsHandler(metrics)

	allowed := []string{"127.0.0.1:1234", "[::1]:1234", "10.1.2.3:1234", "192.168.1.9:9", "172.18.0.2:9"}
	for _, addr := range allowed {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s status %d, want 204", addr, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "8.8.8.8:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("public status %d, want 403", rec.Code)
	}
}

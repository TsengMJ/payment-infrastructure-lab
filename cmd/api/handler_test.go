package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct {
	err error
}

func (f fakePinger) PingContext(context.Context) error { return f.err }

func TestHealthzIgnoresDatabase(t *testing.T) {
	status := get(t, newHandler(fakePinger{err: errors.New("down")}), "/healthz")
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200", status)
	}
}

func TestReadyzFailsWhenDatabasePingFails(t *testing.T) {
	status := get(t, newHandler(fakePinger{err: errors.New("down")}), "/readyz")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", status)
	}
}

func TestReadyzOkWhenDatabasePingSucceeds(t *testing.T) {
	status := get(t, newHandler(fakePinger{}), "/readyz")
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200", status)
	}
}

func TestSlowRouteIsNotPublic(t *testing.T) {
	status := get(t, newHandler(fakePinger{}), "/slow")
	if status != http.StatusNotFound {
		t.Fatalf("status %d, want 404", status)
	}
}

func get(t *testing.T, handler http.Handler, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code
}

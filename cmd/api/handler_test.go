package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestHealthzIgnoresDatabase(t *testing.T) {
	status := get(t, newHandler(fakePinger{err: errors.New("down")}, discardLog(), nil, nil), "/healthz")
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200", status)
	}
}

func TestReadyzFailsWhenDatabasePingFails(t *testing.T) {
	status := get(t, newHandler(fakePinger{err: errors.New("down")}, discardLog(), nil, nil), "/readyz")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", status)
	}
}

func TestReadyzOkWhenDatabasePingSucceeds(t *testing.T) {
	status := get(t, newHandler(fakePinger{}, discardLog(), nil, nil), "/readyz")
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200", status)
	}
}

func TestMetricsIsNotOnThePublicPort(t *testing.T) {
	status := get(t, newHandler(fakePinger{}, discardLog(), nil, nil), "/metrics")
	if status != http.StatusNotFound {
		t.Fatalf("status %d, want 404", status)
	}
}

func TestSlowRouteIsNotPublic(t *testing.T) {
	status := get(t, newHandler(fakePinger{}, discardLog(), nil, nil), "/slow")
	if status != http.StatusNotFound {
		t.Fatalf("status %d, want 404", status)
	}
}

func discardLog() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func get(t *testing.T, handler http.Handler, path string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec.Code
}

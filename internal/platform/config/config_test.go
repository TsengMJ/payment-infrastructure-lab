package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadMissingDatabaseURLNamesTheVariable(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() succeeded without DATABASE_URL")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("error %q does not name DATABASE_URL", err)
	}
}

func TestLoadRejectsMalformedDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "not-a-url")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted a DATABASE_URL without a host")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("error %q does not name DATABASE_URL", err)
	}
}

func TestLoadUsesDefaultsForNonSecrets(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://payment:pw@127.0.0.1:5432/payment_dev?sslmode=disable")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestLoadKeepsExplicitNonSecrets(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://payment:pw@127.0.0.1:5432/payment_dev")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" || cfg.ShutdownTimeout != 3*time.Second || cfg.LogLevel != "debug" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLogValueRedactsDatabaseURL(t *testing.T) {
	const password = "super-secret-pw"
	cfg := Config{
		DatabaseURL:     "postgres://payment:" + password + "@127.0.0.1:5432/payment_dev",
		HTTPAddr:        ":8080",
		ShutdownTimeout: 10 * time.Second,
		LogLevel:        "info",
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("startup", "config", cfg)

	logged := buf.String()
	if strings.Contains(logged, password) {
		t.Fatalf("log contains the database password: %s", logged)
	}
	if strings.Contains(logged, "payment_dev") {
		t.Fatalf("log contains the database URL: %s", logged)
	}
	if !strings.Contains(logged, `"database_url":"***"`) {
		t.Fatalf("database_url was not redacted: %s", logged)
	}
	if !strings.Contains(logged, `"http_addr":":8080"`) {
		t.Fatalf("non-secret field missing: %s", logged)
	}
}

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
	setRequiredConfig(t)
	t.Setenv("DATABASE_URL", "not-a-url")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() accepted a DATABASE_URL without a host")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("error %q does not name DATABASE_URL", err)
	}
}

func TestLoadRejectsPaymentModeAndRedisURL(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("PAYMENT_MODE", "sandbox")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "PAYMENT_MODE") {
		t.Fatalf("PAYMENT_MODE error = %v", err)
	}

	t.Setenv("PAYMENT_MODE", "test")
	t.Setenv("REDIS_URL", "http://127.0.0.1:6379")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "REDIS_URL") {
		t.Fatalf("REDIS_URL error = %v", err)
	}

	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")
	t.Setenv("MANAGEMENT_TOKEN", "too-short")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "MANAGEMENT_TOKEN") {
		t.Fatalf("MANAGEMENT_TOKEN error = %v", err)
	}
}

func TestLoadUsesDefaultsForNonSecrets(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("METRICS_ADDR", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.MetricsAddr != ":8081" {
		t.Fatalf("MetricsAddr = %q, want :8081", cfg.MetricsAddr)
	}
	if cfg.PaymentMode != "test" {
		t.Fatalf("PaymentMode = %q", cfg.PaymentMode)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("ShutdownTimeout = %s, want 10s", cfg.ShutdownTimeout)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestLoadKeepsExplicitNonSecrets(t *testing.T) {
	setRequiredConfig(t)
	t.Setenv("DATABASE_URL", "postgres://payment:pw@127.0.0.1:5432/payment_dev")
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("METRICS_ADDR", "127.0.0.1:9091")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" || cfg.MetricsAddr != "127.0.0.1:9091" || cfg.ShutdownTimeout != 3*time.Second || cfg.LogLevel != "debug" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func setRequiredConfig(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://payment:pw@127.0.0.1:5432/payment_dev?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://127.0.0.1:6379/0")
	t.Setenv("PAYMENT_MODE", "test")
	t.Setenv("MANAGEMENT_TOKEN", "local-dev-management-token-not-secret")
}

func TestLogValueRedactsDatabaseURL(t *testing.T) {
	const password = "super-secret-pw"
	const redisPassword = "redis-secret-pw"
	const managementToken = "local-dev-management-token-not-secret"
	cfg := Config{
		DatabaseURL:     "postgres://payment:" + password + "@127.0.0.1:5432/payment_dev",
		RedisURL:        "redis://:" + redisPassword + "@127.0.0.1:6379/0",
		PaymentMode:     "test",
		ManagementToken: managementToken,
		HTTPAddr:        ":8080",
		MetricsAddr:     ":8081",
		ShutdownTimeout: 10 * time.Second,
		LogLevel:        "info",
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger.Info("startup", "config", cfg)

	logged := buf.String()
	if strings.Contains(logged, password) || strings.Contains(logged, redisPassword) || strings.Contains(logged, managementToken) {
		t.Fatalf("log contains a secret: %s", logged)
	}
	if strings.Contains(logged, "payment_dev") {
		t.Fatalf("log contains the database URL: %s", logged)
	}
	if !strings.Contains(logged, `"database_url":"***"`) || !strings.Contains(logged, `"redis_url":"***"`) || !strings.Contains(logged, `"management_token":"***"`) {
		t.Fatalf("secret fields were not redacted: %s", logged)
	}
	if !strings.Contains(logged, `"http_addr":":8080"`) {
		t.Fatalf("non-secret field missing: %s", logged)
	}
}

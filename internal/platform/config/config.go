// Package config loads process configuration from the environment.
// It does not read files. The composition root may load a local .env
// before calling Load; existing environment variables must win.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

const redacted = "***"

// Config is the process configuration. Fields tagged secret:"true" are
// replaced with redacted when the value is logged.
type Config struct {
	DatabaseURL     string        `secret:"true" json:"database_url"`
	RedisURL        string        `secret:"true" json:"redis_url"`
	PaymentMode     string        `json:"payment_mode"`
	ManagementToken string        `secret:"true" json:"management_token"`
	HTTPAddr        string        `json:"http_addr"`
	MetricsAddr     string        `json:"metrics_addr"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	LogLevel        string        `json:"log_level"`
}

// Load reads configuration from the environment. Required settings have no
// default: a missing DATABASE_URL, REDIS_URL, PAYMENT_MODE, or MANAGEMENT_TOKEN
// is an error that names the variable.
func Load() (Config, error) {
	var missing []string
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		missing = append(missing, "REDIS_URL")
	}
	mode := os.Getenv("PAYMENT_MODE")
	if mode == "" {
		missing = append(missing, "PAYMENT_MODE")
	}
	managementToken := os.Getenv("MANAGEMENT_TOKEN")
	if managementToken == "" {
		missing = append(missing, "MANAGEMENT_TOKEN")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	if err := validateDatabaseURL(databaseURL); err != nil {
		return Config{}, fmt.Errorf("DATABASE_URL: %w", err)
	}
	if err := validateRedisURL(redisURL); err != nil {
		return Config{}, fmt.Errorf("REDIS_URL: %w", err)
	}
	if mode != "test" && mode != "live" {
		return Config{}, fmt.Errorf("PAYMENT_MODE: must be test or live")
	}
	if utf8.RuneCountInString(managementToken) < 32 {
		return Config{}, fmt.Errorf("MANAGEMENT_TOKEN: must be at least 32 characters")
	}

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	metricsAddr := os.Getenv("METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = ":8081"
	}

	timeout := 10 * time.Second
	if raw := os.Getenv("SHUTDOWN_TIMEOUT"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err)
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: must be greater than 0")
		}
		timeout = parsed
	}

	level := os.Getenv("LOG_LEVEL")
	if level == "" {
		level = "info"
	} else if !validLogLevel(level) {
		return Config{}, fmt.Errorf("LOG_LEVEL: must be one of debug, info, warn, error")
	}

	return Config{
		DatabaseURL:     databaseURL,
		RedisURL:        redisURL,
		PaymentMode:     mode,
		ManagementToken: managementToken,
		HTTPAddr:        addr,
		MetricsAddr:     metricsAddr,
		ShutdownTimeout: timeout,
		LogLevel:        level,
	}, nil
}

func validateRedisURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "redis" && u.Scheme != "rediss" {
		return fmt.Errorf("scheme must be redis or rediss")
	}
	if u.Host == "" {
		return fmt.Errorf("must include a host")
	}
	return nil
}

func validateDatabaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("must include a scheme and host")
	}
	return nil
}

func validLogLevel(level string) bool {
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

// LogValue implements slog.LogValuer. Secret fields are logged as ***.
func (c Config) LogValue() slog.Value {
	v := reflect.ValueOf(c)
	t := v.Type()
	attrs := make([]slog.Attr, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := field.Tag.Get("json")
		if name == "" {
			name = field.Name
		}
		value := fmt.Sprint(v.Field(i).Interface())
		if field.Tag.Get("secret") == "true" {
			value = redacted
		}
		attrs = append(attrs, slog.String(name, value))
	}
	return slog.GroupValue(attrs...)
}

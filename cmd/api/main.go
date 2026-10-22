package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/merchant"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/payment"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/clock"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/config"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/httpserver"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/log"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/otel"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/redis"
)

func main() {
	if err := loadEnvFile(); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	level, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	logger := log.New(os.Stdout, "payment-api", level)
	logger.Info("starting", "config", cfg)
	logger.Info("payment mode posture", "payment_mode", cfg.PaymentMode)

	metrics, shutdown, err := otel.Setup(context.Background(), "payment-api")
	if err != nil {
		logger.Error("telemetry", "error", err.Error())
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("database", "error", err.Error())
		_ = shutdown(context.Background())
		os.Exit(1)
	}

	redisClient, err := redis.New(cfg.RedisURL)
	if err != nil {
		logger.Error("redis", "error", err.Error())
		pool.Close()
		_ = shutdown(context.Background())
		os.Exit(1)
	}
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := redisClient.Ping(pingCtx); err != nil {
		logger.Warn("redis unavailable", "error", err.Error())
	}
	pingCancel()

	keys := merchant.NewService(pool, clock.System{})
	public := newHandler(pool, logger, newMerchantAuthenticator(keys), cfg.ManagementToken, keys, payment.NewService(pool))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	runErr := runHTTP(ctx, cfg, public, newMetricsHandler(metrics))

	if err := redisClient.Close(); err != nil {
		logger.Error("redis close", "error", err.Error())
	}
	pool.Close()
	if runErr != nil {
		logger.Error("server", "error", runErr.Error())
		_ = shutdown(context.Background())
		os.Exit(1)
	}
	logger.Info("shutdown")
	if err := shutdown(context.Background()); err != nil {
		logger.Error("telemetry shutdown", "error", err.Error())
		os.Exit(1)
	}
}

func runHTTP(ctx context.Context, cfg config.Config, public, metrics http.Handler) error {
	publicLn, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("http listen: %w", err)
	}
	metricsLn, err := net.Listen("tcp", cfg.MetricsAddr)
	if err != nil {
		_ = publicLn.Close()
		return fmt.Errorf("metrics listen: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	go func() {
		errCh <- httpserver.Serve(runCtx, publicLn, public, cfg.ShutdownTimeout)
	}()
	go func() {
		errCh <- httpserver.Serve(runCtx, metricsLn, metrics, cfg.ShutdownTimeout)
	}()
	first := <-errCh
	cancel()
	second := <-errCh
	if first != nil {
		return first
	}
	return second
}

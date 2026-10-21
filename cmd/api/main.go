package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/config"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/httpserver"
	"github.com/TsengMJ/payment-infrastructure-lab/internal/platform/log"
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

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		logger.Error("database", "error", err.Error())
		os.Exit(1)
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := httpserver.Run(ctx, cfg.HTTPAddr, newHandler(db), cfg.ShutdownTimeout); err != nil {
		logger.Error("server", "error", err.Error())
		os.Exit(1)
	}
	logger.Info("shutdown")
}

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nezzy-joe/image-relevance/internal/config"
	"github.com/Nezzy-joe/image-relevance/internal/database"
	"github.com/Nezzy-joe/image-relevance/internal/providers/gemini"
	"github.com/Nezzy-joe/image-relevance/internal/worker"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "failure", err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)
	provider, err := gemini.New(context.Background(), cfg)
	if err != nil {
		return errors.New("Gemini worker configuration failed; set GEMINI_API_KEY in the local environment")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	connectCtx, stop := context.WithTimeout(ctx, cfg.DatabaseConnect)
	pool, err := database.OpenPool(connectCtx, database.PoolConfig{URL: cfg.DatabaseURL, MaxConns: cfg.DatabaseMaxConns})
	stop()
	if err != nil {
		return errors.New("worker database connection failed; verify database availability and configuration")
	}
	defer pool.Close()
	migrationCtx, stopMigration := context.WithTimeout(ctx, cfg.DatabaseConnect)
	err = database.Migrate(migrationCtx, cfg.DatabaseURL)
	stopMigration()
	if err != nil {
		return errors.New("worker migrations failed; inspect database migration state")
	}
	logger.Info("image processing worker started", "concurrency", cfg.WorkerConcurrency)
	worker.New(pool, provider, cfg.ImageStoragePath, cfg.WorkerConcurrency, cfg.WorkerPoll, cfg.ConfidenceFloor, logger).Run(ctx)
	logger.Info("image processing worker stopped")
	return nil
}

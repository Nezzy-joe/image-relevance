package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/config"
	"github.com/Nezzy-joe/image-relevance/internal/database"
	"github.com/Nezzy-joe/image-relevance/internal/httpapi"
	"github.com/Nezzy-joe/image-relevance/internal/ingestion"
	"github.com/Nezzy-joe/image-relevance/internal/matching"
	"github.com/Nezzy-joe/image-relevance/internal/providers/gemini"
)

func main() {
	if err := run(); err != nil {
		slog.Error("API server stopped", slog.Any("error", err))
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

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), cfg.DatabaseConnect)
	pool, err := database.OpenPool(connectCtx, database.PoolConfig{
		URL:      cfg.DatabaseURL,
		MaxConns: cfg.DatabaseMaxConns,
	})
	cancelConnect()
	if err != nil {
		return errors.New("database initialization failed; verify DATABASE_URL and database availability")
	}
	defer pool.Close()

	migrationCtx, cancelMigration := context.WithTimeout(context.Background(), cfg.DatabaseConnect)
	err = database.Migrate(migrationCtx, cfg.DatabaseURL)
	cancelMigration()
	if err != nil {
		return errors.New("database migrations failed; inspect the migration state and database logs")
	}

	var imageOperations httpapi.ImageOperations
	var matchingOperations httpapi.MatchingOperations
	if cfg.AppEnv == "development" {
		imageOperations = ingestion.NewImageService(pool, cfg.ImageStoragePath, cfg.MaxImageBytes)
		if cfg.GeminiAPIKey != "" {
			provider, err := gemini.New(context.Background(), cfg)
			if err != nil {
				return errors.New("Gemini client initialization failed")
			}
			matchingOperations = matching.New(pool, provider, cfg.ConfidenceFloor, cfg.SimilarityFloor)
		}
	}
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouterWithOperations(logger, pool, cfg.DatabaseReadiness, imageOperations, matchingOperations, cfg.MaxImageBytes),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("API server listening", slog.String("address", cfg.HTTPAddr), slog.String("environment", cfg.AppEnv))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case <-shutdownCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("graceful HTTP shutdown: %w", err)
		}
		logger.Info("API server shut down")
		return nil
	case err := <-serverErrors:
		return fmt.Errorf("serve HTTP: %w", err)
	}
}

package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

// Keep this aligned with the existing Compose database and persistent volume.
const localDevelopmentDatabaseURL = "postgres://flyrank:flyrank_dev_password@localhost:15432/flyrank?sslmode=disable"

type Config struct {
	AppEnv            string
	HTTPAddr          string
	DatabaseURL       string
	DatabaseConnect   time.Duration
	DatabaseReadiness time.Duration
	DatabaseMaxConns  int32
	ShutdownTimeout   time.Duration
	GeminiAPIKey      string
	VisionModel       string
	EmbeddingModel    string
	ImageStoragePath  string
	MaxImageBytes     int64
	WorkerConcurrency int
	WorkerPoll        time.Duration
	ConfidenceFloor   float64
	SimilarityFloor   float64
	LogLevel          slog.Level
}

func Load() (Config, error) {
	appEnv := strings.ToLower(envString("APP_ENV", "development"))
	switch appEnv {
	case "development", "test", "production":
	default:
		return Config{}, fmt.Errorf("unsupported APP_ENV %q; must be development, test, or production", appEnv)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if appEnv == "production" && databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required when APP_ENV=production")
	}
	if databaseURL == "" {
		databaseURL = envString("DATABASE_URL_DEVELOPMENT_DEFAULT", localDevelopmentDatabaseURL)
	}

	cfg := Config{
		AppEnv:            appEnv,
		HTTPAddr:          envString("HTTP_ADDR", ":8080"),
		DatabaseURL:       databaseURL,
		DatabaseMaxConns:  10,
		GeminiAPIKey:      strings.TrimSpace(os.Getenv("GEMINI_API_KEY")),
		VisionModel:       envString("GEMINI_VISION_MODEL", "gemini-3.1-flash-lite"),
		EmbeddingModel:    envString("GEMINI_EMBEDDING_MODEL", "gemini-embedding-2"),
		ImageStoragePath:  envString("IMAGE_STORAGE_PATH", "./data/images"),
		WorkerConcurrency: 2,
		ConfidenceFloor:   0.65,
		SimilarityFloor:   0.35,
	}

	var err error
	if cfg.DatabaseConnect, err = envDuration("DB_CONNECT_TIMEOUT", 5*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseReadiness, err = envDuration("DB_READINESS_TIMEOUT", 2*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = envDuration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseMaxConns, err = envInt32("DB_MAX_CONNS", 10); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseMaxConns < 1 {
		return Config{}, fmt.Errorf("DB_MAX_CONNS must be at least 1")
	}
	if cfg.MaxImageBytes, err = envInt64("MAX_IMAGE_BYTES", 10<<20); err != nil {
		return Config{}, err
	}
	if cfg.MaxImageBytes < 1 {
		return Config{}, fmt.Errorf("MAX_IMAGE_BYTES must be at least 1")
	}
	if cfg.WorkerConcurrency, err = envInt("WORKER_CONCURRENCY", 2); err != nil {
		return Config{}, err
	}
	if cfg.WorkerConcurrency < 1 || cfg.WorkerConcurrency > 32 {
		return Config{}, fmt.Errorf("WORKER_CONCURRENCY must be between 1 and 32")
	}
	if cfg.WorkerPoll, err = envDuration("WORKER_POLL_INTERVAL", 500*time.Millisecond); err != nil {
		return Config{}, err
	}
	if cfg.ConfidenceFloor, err = envFloat64("VISION_CONFIDENCE_FLOOR", 0.65); err != nil {
		return Config{}, err
	}
	if cfg.ConfidenceFloor < 0 || cfg.ConfidenceFloor > 1 {
		return Config{}, fmt.Errorf("VISION_CONFIDENCE_FLOOR must be between 0 and 1")
	}
	if cfg.SimilarityFloor, err = envFloat64("MATCH_SIMILARITY_FLOOR", 0.35); err != nil {
		return Config{}, err
	}
	if cfg.SimilarityFloor < -1 || cfg.SimilarityFloor > 1 {
		return Config{}, fmt.Errorf("MATCH_SIMILARITY_FLOOR must be between -1 and 1")
	}
	if cfg.LogLevel, err = parseLogLevel(envString("LOG_LEVEL", "info")); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return duration, nil
}

func envInt32(key string, fallback int32) (int32, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return int32(parsed), nil
}

func envInt(key string, fallback int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}

func envInt64(key string, fallback int64) (int64, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}

func envFloat64(key string, fallback float64) (float64, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", key)
	}
	return parsed, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL must be debug, info, warn, or error")
	}
}

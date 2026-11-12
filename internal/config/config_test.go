package config

import (
	"strings"
	"testing"
)

func TestLoadDevelopmentDefaults(t *testing.T) {
	clearConfigEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv = %q, want development", cfg.AppEnv)
	}
	if cfg.DatabaseURL != localDevelopmentDatabaseURL {
		t.Errorf("DatabaseURL = %q, want documented local default", cfg.DatabaseURL)
	}
	if cfg.VisionModel != "gemini-3.1-flash-lite" || cfg.EmbeddingModel != "gemini-embedding-2" {
		t.Errorf("unexpected Gemini model defaults: vision=%q embedding=%q", cfg.VisionModel, cfg.EmbeddingModel)
	}
	if cfg.GeminiAPIKey != "" {
		t.Error("GeminiAPIKey should remain unset by default")
	}
}

func TestLoadProductionWithExplicitDatabaseURL(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("APP_ENV", "production")
	const databaseURL = "postgres://app_user@postgres.internal:5432/flyrank?sslmode=verify-full"
	t.Setenv("DATABASE_URL", databaseURL)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppEnv != "production" {
		t.Errorf("AppEnv = %q, want production", cfg.AppEnv)
	}
	if cfg.DatabaseURL != databaseURL {
		t.Errorf("DatabaseURL = %q, want explicit DATABASE_URL", cfg.DatabaseURL)
	}
}

func TestLoadProductionRequiresExplicitDatabaseURL(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("DATABASE_URL_DEVELOPMENT_DEFAULT", localDevelopmentDatabaseURL)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want missing DATABASE_URL error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL is required when APP_ENV=production") {
		t.Fatalf("Load() error = %q, want clear production DATABASE_URL error", err)
	}
	if strings.Contains(err.Error(), "flyrank_dev_password") {
		t.Fatalf("configuration error exposed a development password: %v", err)
	}
}

func TestLoadRejectsUnsupportedEnvironment(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("APP_ENV", "staging")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want unsupported APP_ENV error")
	}
	if !strings.Contains(err.Error(), `unsupported APP_ENV "staging"`) {
		t.Fatalf("Load() error = %q, want unsupported APP_ENV error", err)
	}
}

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"APP_ENV",
		"HTTP_ADDR",
		"DATABASE_URL",
		"DATABASE_URL_DEVELOPMENT_DEFAULT",
		"DB_CONNECT_TIMEOUT",
		"DB_READINESS_TIMEOUT",
		"DB_MAX_CONNS",
		"SHUTDOWN_TIMEOUT",
		"LOG_LEVEL",
		"GEMINI_API_KEY",
		"GEMINI_VISION_MODEL",
		"GEMINI_EMBEDDING_MODEL",
		"IMAGE_STORAGE_PATH",
		"MAX_IMAGE_BYTES",
		"WORKER_CONCURRENCY",
		"WORKER_POLL_INTERVAL",
		"VISION_CONFIDENCE_FLOOR",
		"MATCH_SIMILARITY_FLOOR",
	} {
		t.Setenv(name, "")
	}
}

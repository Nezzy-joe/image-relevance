package config

import (
	"strings"
	"testing"
	"time"
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
	if cfg.HTTPAddr != "127.0.0.1:8080" {
		t.Errorf("HTTPAddr = %q, want host-loopback default", cfg.HTTPAddr)
	}
	if cfg.HTTPWriteTimeout != 65*time.Second || cfg.GeminiRequestTimeout != 25*time.Second {
		t.Errorf("timeout defaults = HTTP %s / Gemini %s, want 65s / 25s", cfg.HTTPWriteTimeout, cfg.GeminiRequestTimeout)
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

func TestLoadPreservesExplicitHTTPAddressAndTimeouts(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("HTTP_ADDR", "0.0.0.0:8080")
	t.Setenv("HTTP_WRITE_TIMEOUT", "75s")
	t.Setenv("GEMINI_REQUEST_TIMEOUT", "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != "0.0.0.0:8080" {
		t.Errorf("HTTPAddr = %q, want explicit listener", cfg.HTTPAddr)
	}
	if cfg.HTTPWriteTimeout != 75*time.Second || cfg.GeminiRequestTimeout != 30*time.Second {
		t.Errorf("timeouts = HTTP %s / Gemini %s, want 75s / 30s", cfg.HTTPWriteTimeout, cfg.GeminiRequestTimeout)
	}
}

func TestLoadRejectsInsufficientHTTPWriteTimeoutBudget(t *testing.T) {
	clearConfigEnvironment(t)
	t.Setenv("HTTP_WRITE_TIMEOUT", "55s")
	t.Setenv("GEMINI_REQUEST_TIMEOUT", "25s")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "two GEMINI_REQUEST_TIMEOUT calls plus 10s response margin") {
		t.Fatalf("Load() error = %v, want insufficient provider/HTTP timeout budget", err)
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
		"HTTP_WRITE_TIMEOUT",
		"GEMINI_REQUEST_TIMEOUT",
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

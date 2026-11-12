package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/matching"
	"github.com/Nezzy-joe/image-relevance/internal/middleware"
	"github.com/go-chi/chi/v5"
)

type DatabasePinger interface {
	Ping(context.Context) error
}

func NewRouter(logger *slog.Logger, database DatabasePinger, readinessTimeout time.Duration) http.Handler {
	return NewRouterWithImages(logger, database, readinessTimeout, nil, 10<<20)
}

type ImageOperations interface {
	Upload(context.Context, []byte, string) (domain.ImageUploadResult, error)
	Get(context.Context, string) (domain.ImageRecord, error)
}

func NewRouterWithImages(logger *slog.Logger, database DatabasePinger, readinessTimeout time.Duration, images ImageOperations, maxImageBytes int64) http.Handler {
	return NewRouterWithOperations(logger, database, readinessTimeout, images, nil, maxImageBytes)
}

type MatchingOperations interface {
	CreatePost(context.Context, matching.PostInput) (matching.Post, error)
	Match(context.Context, string) (matching.MatchResult, error)
	Review(context.Context, string, matching.Review) (matching.ReviewResult, error)
}

func NewRouterWithOperations(logger *slog.Logger, database DatabasePinger, readinessTimeout time.Duration, images ImageOperations, matches MatchingOperations, maxImageBytes int64) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.AccessLog(logger))
	router.Use(middleware.Recover(logger))
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusNotFound, "not_found", "the requested resource was not found", middleware.RequestIDFromContext(r.Context()))
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "the method is not allowed for this resource", middleware.RequestIDFromContext(r.Context()))
	})

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
	router.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if database == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
				"checks": map[string]string{"database": "unavailable"},
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()
		if err := database.Ping(ctx); err != nil {
			logger.WarnContext(r.Context(), "readiness database check failed",
				slog.String("request_id", middleware.RequestIDFromContext(r.Context())),
				slog.String("failure", "database_ping_failed"),
			)
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"status": "not_ready",
				"checks": map[string]string{"database": "unavailable"},
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ready",
			"checks": map[string]string{"database": "ok"},
		})
	})
	if images != nil {
		registerImageRoutes(router, images, maxImageBytes)
	}
	if matches != nil {
		registerMatchingRoutes(router, matches)
	}

	return router
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeJSONError(w http.ResponseWriter, status int, code, message, requestID string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]string{
			"code":       code,
			"message":    message,
			"request_id": requestID,
		},
	})
}

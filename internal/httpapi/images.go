package httpapi

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/middleware"
	"github.com/go-chi/chi/v5"
)

const multipartOverheadLimit = 1 << 20

var imageIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func registerImageRoutes(router chi.Router, images ImageOperations, maxImageBytes int64) {
	router.Route("/v1/images", func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			filter, err := parseImageListFilter(r)
			if err != nil {
				writeImageJSONError(w, r, http.StatusBadRequest, "invalid_query", "image filters or pagination are invalid")
				return
			}
			page, err := images.List(r.Context(), filter)
			if err != nil {
				writeImageError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, page)
		})
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			uploadImage(w, r, images, maxImageBytes)
		})
		r.Get("/{imageID}/content", func(w http.ResponseWriter, r *http.Request) {
			imageID := chi.URLParam(r, "imageID")
			if !imageIDPattern.MatchString(imageID) {
				writeImageError(w, r, domain.ErrImageNotFound)
				return
			}
			content, err := images.Content(r.Context(), imageID)
			if err != nil {
				writeImageError(w, r, err)
				return
			}
			defer content.Body.Close()
			w.Header().Set("Content-Type", content.MIMEType)
			w.Header().Set("Content-Length", strconv.FormatInt(content.SizeBytes, 10))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "private, no-store")
			w.Header().Set("Content-Disposition", "inline")
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, content.Body)
		})
		r.Get("/{imageID}", func(w http.ResponseWriter, r *http.Request) {
			imageID := chi.URLParam(r, "imageID")
			if !imageIDPattern.MatchString(imageID) {
				writeImageError(w, r, domain.ErrImageNotFound)
				return
			}
			image, err := images.Get(r.Context(), imageID)
			if err != nil {
				writeImageError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, image)
		})
	})
}

func parseImageListFilter(r *http.Request) (domain.ImageListFilter, error) {
	query := r.URL.Query()
	filter := domain.ImageListFilter{Limit: 24, Offset: 0}
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 100 {
			return domain.ImageListFilter{}, errors.New("invalid limit")
		}
		filter.Limit = parsed
	}
	if value := query.Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > 1_000_000 {
			return domain.ImageListFilter{}, errors.New("invalid offset")
		}
		filter.Offset = parsed
	}
	filter.Query = query.Get("q")
	filter.Status = query.Get("status")
	filter.Category = query.Get("category")
	filter.Subject = query.Get("subject")
	if len(filter.Query) > 120 || len(filter.Category) > 64 || len(filter.Subject) > 120 {
		return domain.ImageListFilter{}, errors.New("filter is too long")
	}
	switch filter.Status {
	case "", "queued", "processing", "processed", "needs_review", "failed":
	default:
		return domain.ImageListFilter{}, errors.New("invalid status")
	}
	return filter, nil
}

func uploadImage(w http.ResponseWriter, r *http.Request, images ImageOperations, maxImageBytes int64) {
	if maxImageBytes <= 0 {
		maxImageBytes = 10 << 20
	}
	if r.ContentLength > 0 && r.ContentLength > maxImageBytes+multipartOverheadLimit {
		writeImageJSONError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "the uploaded image exceeds the configured limit")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+multipartOverheadLimit)
	file, header, err := r.FormFile("file")
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeImageJSONError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "the uploaded image exceeds the configured limit")
		} else {
			writeImageJSONError(w, r, http.StatusBadRequest, "invalid_upload", "multipart field 'file' is required")
		}
		return
	}
	defer file.Close()
	if header.Size > maxImageBytes {
		writeImageJSONError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "the uploaded image exceeds the configured limit")
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		writeImageJSONError(w, r, http.StatusBadRequest, "invalid_upload", "the uploaded image could not be read")
		return
	}
	if int64(len(data)) > maxImageBytes {
		writeImageJSONError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "the uploaded image exceeds the configured limit")
		return
	}
	result, err := images.Upload(r.Context(), data, header.Header.Get("Content-Type"))
	if err != nil {
		writeImageError(w, r, err)
		return
	}
	if result.Existing {
		writeJSON(w, http.StatusOK, result)
		return
	}
	w.Header().Set("Location", "/v1/images/"+result.ImageID)
	writeJSON(w, http.StatusAccepted, result)
}

func writeImageError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrImageTooLarge):
		writeImageJSONError(w, r, http.StatusRequestEntityTooLarge, "payload_too_large", "the uploaded image exceeds the configured limit")
	case errors.Is(err, domain.ErrInvalidImage):
		writeImageJSONError(w, r, http.StatusBadRequest, "invalid_image", "only valid JPEG and PNG images are accepted")
	case errors.Is(err, domain.ErrImageNotFound):
		writeImageJSONError(w, r, http.StatusNotFound, "not_found", "the requested image was not found")
	case errors.Is(err, domain.ErrNoTenant):
		writeImageJSONError(w, r, http.StatusServiceUnavailable, "tenant_not_seeded", "run the development seed command before using image routes")
	default:
		writeImageJSONError(w, r, http.StatusInternalServerError, "image_operation_failed", "the image operation could not be completed")
	}
}

func writeImageJSONError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSONError(w, status, code, message, middleware.RequestIDFromContext(r.Context()))
}

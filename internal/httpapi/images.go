package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/middleware"
	"github.com/go-chi/chi/v5"
)

const multipartOverheadLimit = 1 << 20

func registerImageRoutes(router chi.Router, images ImageOperations, maxImageBytes int64) {
	router.Route("/v1/images", func(r chi.Router) {
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			uploadImage(w, r, images, maxImageBytes)
		})
		r.Get("/{imageID}", func(w http.ResponseWriter, r *http.Request) {
			image, err := images.Get(r.Context(), chi.URLParam(r, "imageID"))
			if err != nil {
				writeImageError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, image)
		})
	})
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
		writeImageJSONError(w, r, http.StatusServiceUnavailable, "tenant_not_seeded", "run the development seed command before uploading")
	default:
		writeImageJSONError(w, r, http.StatusInternalServerError, "image_operation_failed", "the image operation could not be completed")
	}
}

func writeImageJSONError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSONError(w, status, code, message, middleware.RequestIDFromContext(r.Context()))
}

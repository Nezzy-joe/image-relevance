package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/matching"
	"github.com/go-chi/chi/v5"
)

func registerMatchingRoutes(router chi.Router, operations MatchingOperations) {
	router.Route("/v1", func(r chi.Router) {
		r.Post("/posts", func(w http.ResponseWriter, r *http.Request) {
			var input matching.PostInput
			if err := decodeJSON(w, r, 22<<10, &input); err != nil {
				writeImageJSONError(w, r, http.StatusBadRequest, "invalid_request", "request body must be a valid post JSON object")
				return
			}
			post, err := operations.CreatePost(r.Context(), input)
			if err != nil {
				writeMatchingError(w, r, err)
				return
			}
			writeJSON(w, http.StatusCreated, post)
		})
		r.Post("/posts/{postID}/matches", func(w http.ResponseWriter, r *http.Request) {
			result, err := operations.Match(r.Context(), chi.URLParam(r, "postID"))
			if err != nil {
				writeMatchingError(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, result)
		})
		r.Post("/matches/{suggestionID}/review", func(w http.ResponseWriter, r *http.Request) {
			var input matching.Review
			if err := decodeJSON(w, r, 4<<10, &input); err != nil {
				writeImageJSONError(w, r, http.StatusBadRequest, "invalid_request", "request body must be a valid review JSON object")
				return
			}
			result, err := operations.Review(r.Context(), chi.URLParam(r, "suggestionID"), input)
			if err != nil {
				writeMatchingError(w, r, err)
				return
			}
			writeJSON(w, http.StatusCreated, result)
		})
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("request must contain a single JSON value")
	}
	return nil
}

func writeMatchingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, matching.ErrPostNotFound), errors.Is(err, matching.ErrSuggestionNotFound):
		writeImageJSONError(w, r, http.StatusNotFound, "not_found", "the requested post or match suggestion was not found")
	case errors.Is(err, matching.ErrInvalidReview), errors.Is(err, matching.ErrReviewTooLong):
		writeImageJSONError(w, r, http.StatusBadRequest, "invalid_review", "decision must be approved or rejected and comments must be at most 2000 characters")
	case errors.Is(err, matching.ErrInvalidPost):
		writeImageJSONError(w, r, http.StatusBadRequest, "invalid_post", "title and body are required and must fit the documented limits")
	case errors.Is(err, domain.ErrNoTenant):
		writeImageJSONError(w, r, http.StatusServiceUnavailable, "tenant_not_seeded", "run the development seed command before using matching routes")
	default:
		writeImageJSONError(w, r, http.StatusInternalServerError, "matching_operation_failed", "the matching operation could not be completed")
	}
}

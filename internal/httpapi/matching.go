package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

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
	})
}

func registerReviewRoute(router chi.Router, operations ReviewOperations) {
	router.Post("/v1/matches/{suggestionID}/review", func(w http.ResponseWriter, r *http.Request) {
		suggestionID := chi.URLParam(r, "suggestionID")
		if !imageIDPattern.MatchString(suggestionID) {
			writeImageJSONError(w, r, http.StatusNotFound, "not_found", "the requested match suggestion was not found")
			return
		}
		var input matching.Review
		if err := decodeJSON(w, r, 4<<10, &input); err != nil {
			writeImageJSONError(w, r, http.StatusBadRequest, "invalid_request", "request body must be a valid review JSON object")
			return
		}
		result, err := operations.Review(r.Context(), suggestionID, input)
		if err != nil {
			writeMatchingError(w, r, err)
			return
		}
		writeJSON(w, http.StatusCreated, result)
	})
}

func registerMatchHistoryRoute(router chi.Router, operations MatchHistoryOperations) {
	router.Get("/v1/matches", func(w http.ResponseWriter, r *http.Request) {
		limit, offset, err := parsePagination(r, 20, 100)
		if err != nil {
			writeImageJSONError(w, r, http.StatusBadRequest, "invalid_query", "pagination is invalid")
			return
		}
		page, err := operations.ListMatches(r.Context(), limit, offset)
		if err != nil {
			writeMatchingError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	})
	router.Get("/v1/matches/{suggestionID}/reviews", func(w http.ResponseWriter, r *http.Request) {
		suggestionID := chi.URLParam(r, "suggestionID")
		if !imageIDPattern.MatchString(suggestionID) {
			writeImageJSONError(w, r, http.StatusNotFound, "not_found", "the requested match suggestion was not found")
			return
		}
		limit, offset, err := parsePagination(r, 20, 100)
		if err != nil {
			writeImageJSONError(w, r, http.StatusBadRequest, "invalid_query", "pagination is invalid")
			return
		}
		page, err := operations.ListReviewHistory(r.Context(), suggestionID, limit, offset)
		if err != nil {
			writeMatchingError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, page)
	})
}

func parsePagination(r *http.Request, defaultLimit, maxLimit int) (int, int, error) {
	limit, offset := defaultLimit, 0
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > maxLimit {
			return 0, 0, errors.New("invalid limit")
		}
		limit = parsed
	}
	if value := r.URL.Query().Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > 1_000_000 {
			return 0, 0, errors.New("invalid offset")
		}
		offset = parsed
	}
	return limit, offset, nil
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

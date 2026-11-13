package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/matching"
)

type fakePinger struct{ err error }

func (p fakePinger) Ping(context.Context) error { return p.err }

func TestHealthIsIndependentOfDatabase(t *testing.T) {
	response := performRequest(NewRouter(testLogger(), nil, time.Second), "/health")
	body, _ := io.ReadAll(response.Body)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("body = %s, want health status", body)
	}
	if got := response.Header().Get("X-Request-ID"); got == "" {
		t.Fatal("X-Request-ID header is empty")
	}
}

func TestReadyReportsDatabaseState(t *testing.T) {
	tests := []struct {
		name       string
		pinger     fakePinger
		wantStatus int
		wantBody   string
	}{
		{name: "ready", pinger: fakePinger{}, wantStatus: http.StatusOK, wantBody: `"status":"ready"`},
		{name: "database unavailable", pinger: fakePinger{err: errors.New("private connection details")}, wantStatus: http.StatusServiceUnavailable, wantBody: `"database":"unavailable"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := performRequest(NewRouter(testLogger(), tt.pinger, time.Second), "/ready")
			body, _ := io.ReadAll(response.Body)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if !strings.Contains(string(body), tt.wantBody) {
				t.Fatalf("body = %s, want fragment %s", body, tt.wantBody)
			}
			if strings.Contains(string(body), "private connection details") {
				t.Fatalf("database error leaked to response: %s", body)
			}
		})
	}
}

func TestReadinessDoesNotLogDatabaseConnectionString(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	connectionString := "postgres://app_user:private-password@db.internal:5432/flyrank?sslmode=verify-full"
	handler := NewRouter(logger, fakePinger{err: errors.New(connectionString)}, time.Second)
	response := performRequest(handler, "/ready")

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if strings.Contains(logs.String(), connectionString) || strings.Contains(logs.String(), "private-password") {
		t.Fatalf("database connection string was logged: %s", logs.String())
	}
}

func TestUnknownRouteUsesJSONError(t *testing.T) {
	response := performRequest(NewRouter(testLogger(), nil, time.Second), "/missing")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("body = %s, want not_found error", response.Body.String())
	}
}

type fakeImageOperations struct {
	page    domain.ImagePage
	filter  domain.ImageListFilter
	content domain.ImageContent
	err     error
}

func (f *fakeImageOperations) Upload(context.Context, []byte, string) (domain.ImageUploadResult, error) {
	return domain.ImageUploadResult{}, nil
}
func (f *fakeImageOperations) Get(context.Context, string) (domain.ImageRecord, error) {
	return domain.ImageRecord{}, f.err
}
func (f *fakeImageOperations) List(_ context.Context, filter domain.ImageListFilter) (domain.ImagePage, error) {
	f.filter = filter
	return f.page, f.err
}
func (f *fakeImageOperations) Content(context.Context, string) (domain.ImageContent, error) {
	return f.content, f.err
}

type fakeMatchingOperations struct {
	page matching.MatchPage
	err  error
}

func (f fakeMatchingOperations) CreatePost(context.Context, matching.PostInput) (matching.Post, error) {
	return matching.Post{}, f.err
}
func (f fakeMatchingOperations) Match(context.Context, string) (matching.MatchResult, error) {
	return matching.MatchResult{}, f.err
}
func (f fakeMatchingOperations) Review(context.Context, string, matching.Review) (matching.ReviewResult, error) {
	return matching.ReviewResult{}, f.err
}
func (f fakeMatchingOperations) ListMatches(context.Context, int, int) (matching.MatchPage, error) {
	return f.page, f.err
}
func (f fakeMatchingOperations) ListReviewHistory(context.Context, string, int, int) (matching.ReviewHistoryPage, error) {
	return matching.ReviewHistoryPage{Items: []matching.ReviewResult{}, Limit: 20}, f.err
}

func TestImageListParsesFiltersAndReturnsPage(t *testing.T) {
	images := &fakeImageOperations{page: domain.ImagePage{Items: []domain.ImageRecord{}, Total: 8, Limit: 10, Offset: 20}}
	handler := NewRouterWithImages(testLogger(), fakePinger{}, time.Second, images, 10<<20)
	response := performRequest(handler, "/v1/images?limit=10&offset=20&q=red&category=wildlife&subject=fox&status=processed")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"total":8`) {
		t.Fatalf("image list response = (%d, %s)", response.Code, response.Body.String())
	}
	if images.filter.Limit != 10 || images.filter.Offset != 20 || images.filter.Query != "red" || images.filter.Category != "wildlife" || images.filter.Subject != "fox" || images.filter.Status != "processed" {
		t.Fatalf("unexpected filter passed to image service: %+v", images.filter)
	}
}

func TestImageListRejectsInvalidPaginationAndStatus(t *testing.T) {
	for _, path := range []string{"/v1/images?limit=0", "/v1/images?offset=-1", "/v1/images?status=unknown"} {
		response := performRequest(NewRouterWithImages(testLogger(), fakePinger{}, time.Second, &fakeImageOperations{}, 10<<20), path)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_query"`) {
			t.Errorf("%s response = (%d, %s), want invalid query", path, response.Code, response.Body.String())
		}
	}
}

func TestImageContentUsesSafeHeadersAndStreamsBody(t *testing.T) {
	images := &fakeImageOperations{content: domain.ImageContent{Body: io.NopCloser(strings.NewReader("jpeg-data")), MIMEType: "image/jpeg", SizeBytes: 9}}
	response := performRequest(NewRouterWithImages(testLogger(), fakePinger{}, time.Second, images, 10<<20), "/v1/images/11111111-1111-4111-8111-111111111111/content")
	if response.Code != http.StatusOK || response.Body.String() != "jpeg-data" {
		t.Fatalf("content response = (%d, %q)", response.Code, response.Body.String())
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("unsafe or missing content headers: %v", response.Header())
	}
}

func TestImageContentRejectsMalformedImageIDAsNotFound(t *testing.T) {
	response := performRequest(NewRouterWithImages(testLogger(), fakePinger{}, time.Second, &fakeImageOperations{}, 10<<20), "/v1/images/not-a-uuid/content")
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("malformed image ID response=(%d, %s), want safe 404", response.Code, response.Body.String())
	}
}

func TestRecentMatchesRouteReturnsPersistedPage(t *testing.T) {
	handler := NewRouterWithOperations(testLogger(), fakePinger{}, time.Second, nil, fakeMatchingOperations{
		page: matching.MatchPage{Items: []matching.RecentMatch{}, Total: 3, Limit: 2, Offset: 1},
	}, 10<<20)
	response := performRequest(handler, "/v1/matches?limit=2&offset=1")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"total":3`) {
		t.Fatalf("matches response = (%d, %s)", response.Code, response.Body.String())
	}
}

func TestRecentMatchesCanBeReadWithoutGeminiOperations(t *testing.T) {
	handler := NewRouterWithReadOperations(testLogger(), fakePinger{}, time.Second, nil, nil, fakeMatchingOperations{
		page: matching.MatchPage{Items: []matching.RecentMatch{}, Total: 4, Limit: 20, Offset: 0},
	}, 10<<20)
	response := performRequest(handler, "/v1/matches")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"total":4`) {
		t.Fatalf("history without Gemini response = (%d, %s)", response.Code, response.Body.String())
	}
}

func TestReviewHistoryRouteReturnsTenantScopedHistoryPage(t *testing.T) {
	handler := NewRouterWithReadOperations(testLogger(), fakePinger{}, time.Second, nil, nil, fakeMatchingOperations{}, 10<<20)
	response := performRequest(handler, "/v1/matches/11111111-1111-4111-8111-111111111111/reviews?limit=10&offset=2")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"items":[]`) {
		t.Fatalf("review history response = (%d, %s)", response.Code, response.Body.String())
	}
}

func TestReviewHistoryRejectsMalformedSuggestionID(t *testing.T) {
	handler := NewRouterWithReadOperations(testLogger(), fakePinger{}, time.Second, nil, nil, fakeMatchingOperations{}, 10<<20)
	response := performRequest(handler, "/v1/matches/nope/reviews")
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) {
		t.Fatalf("malformed history ID response = (%d, %s)", response.Code, response.Body.String())
	}
}

func TestReviewActionIsAvailableWithoutGeminiMatchingOperations(t *testing.T) {
	handler := NewRouterWithReadOperations(testLogger(), fakePinger{}, time.Second, nil, nil, fakeMatchingOperations{}, 10<<20)
	request := httptest.NewRequest(http.MethodPost, "/v1/matches/11111111-1111-4111-8111-111111111111/review", strings.NewReader(`{"decision":"approved"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("review response = (%d, %s), want 201 without Gemini operations", response.Code, response.Body.String())
	}
}

func performRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

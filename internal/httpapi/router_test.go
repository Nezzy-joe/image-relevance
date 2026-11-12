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

func performRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

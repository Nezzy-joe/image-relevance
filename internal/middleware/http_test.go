package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecoverReturnsSafeJSONError(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	panicHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("private database detail")
	})
	handler := RequestID(Recover(logger)(panicHandler))
	request := httptest.NewRequest(http.MethodGet, "/panic", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	var payload struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != "internal_error" || payload.Error.RequestID == "" || payload.Error.Message == "" {
		t.Fatalf("unexpected error payload: %+v", payload.Error)
	}
	if !json.Valid(response.Body.Bytes()) {
		t.Fatalf("response is not valid JSON: %q", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "private database detail") {
		t.Fatalf("panic detail leaked into response: %s", response.Body.String())
	}
	if strings.Contains(logs.String(), "private database detail") {
		t.Fatalf("panic detail leaked into logs: %s", logs.String())
	}
}

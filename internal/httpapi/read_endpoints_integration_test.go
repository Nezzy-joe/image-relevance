package httpapi

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/database"
	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/ingestion"
	"github.com/Nezzy-joe/image-relevance/internal/matching"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This integration test is intentionally opt-in and rejects database names
// without "test". Point it only at a disposable PostgreSQL database.
func TestReadEndpointsAgainstDisposablePostgres(t *testing.T) {
	databaseURL := os.Getenv("IMAGE_RELEVANCE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set IMAGE_RELEVANCE_TEST_DATABASE_URL to an isolated database whose name includes 'test'")
	}
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal("test database URL is invalid")
	}
	if !strings.Contains(strings.ToLower(poolConfig.ConnConfig.Database), "test") {
		t.Fatal("refusing integration test database whose name does not include 'test'")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, databaseURL); err != nil {
		t.Fatal("could not apply migrations to the disposable test database")
	}
	pool, err := database.OpenPool(ctx, database.PoolConfig{URL: databaseURL, MaxConns: 2})
	if err != nil {
		t.Fatal("could not connect to the disposable test database")
	}
	defer pool.Close()
	const tenantName = "image-relevance development tenant"
	var tenantCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tenants WHERE name=$1`, tenantName).Scan(&tenantCount); err != nil {
		t.Fatal("could not inspect the test tenant")
	}
	if tenantCount != 0 {
		t.Fatal("disposable test database already contains a development tenant; use a fresh database")
	}
	var tenantID string
	if err := pool.QueryRow(ctx, `INSERT INTO tenants (name) VALUES ($1) RETURNING id::text`, tenantName).Scan(&tenantID); err != nil {
		t.Fatal("could not create the test tenant")
	}
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1::uuid`, tenantID) }()

	var encoded bytes.Buffer
	fixture := image.NewRGBA(image.Rect(0, 0, 4, 4))
	fixture.Set(1, 1, color.RGBA{R: 220, G: 100, B: 20, A: 255})
	if err := png.Encode(&encoded, fixture); err != nil {
		t.Fatal("could not create PNG test fixture")
	}
	imageService := ingestion.NewImageService(pool, t.TempDir(), 10<<20)
	upload, err := imageService.Upload(ctx, encoded.Bytes(), "image/png")
	if err != nil {
		t.Fatal("could not persist test image")
	}
	_, err = pool.Exec(ctx, `UPDATE images SET status='processed' WHERE id=$1::uuid AND tenant_id=$2::uuid`, upload.ImageID, tenantID)
	if err != nil {
		t.Fatal("could not set test image state")
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO image_analysis (image_id, subject, subject_key, category, attributes, caption, confidence, uncertain, model)
		VALUES ($1::uuid, 'red fox', 'red fox', 'wildlife', '["russet fur"]'::jsonb,
		        'A red fox in a meadow.', 0.94, false, 'integration-test')`, upload.ImageID)
	if err != nil {
		t.Fatal("could not prepare test image metadata")
	}
	page, err := imageService.List(ctx, domain.ImageListFilter{Limit: 24})
	if err != nil || page.Total != 1 || page.StatusCounts["processed"] != 1 || len(page.Items) != 1 || page.Items[0].Metadata == nil {
		t.Fatalf("image listing did not return persisted metadata and counts (total=%d, err=%v)", page.Total, err)
	}
	content, err := imageService.Content(ctx, upload.ImageID)
	if err != nil {
		t.Fatal("could not open test image content")
	}
	data, err := io.ReadAll(content.Body)
	_ = content.Body.Close()
	if err != nil || !bytes.Equal(data, encoded.Bytes()) || content.MIMEType != "image/png" {
		t.Fatal("image content endpoint service returned unexpected bytes or content type")
	}
	imageRouter := NewRouterWithImages(testLogger(), pool, time.Second, imageService, 10<<20)
	imageResponse := performRequest(imageRouter, "/v1/images?limit=24")
	if imageResponse.Code != http.StatusOK || !strings.Contains(imageResponse.Body.String(), `"status":"processed"`) {
		t.Fatalf("image HTTP list response=(%d, %s)", imageResponse.Code, imageResponse.Body.String())
	}
	contentResponse := performRequest(imageRouter, "/v1/images/"+upload.ImageID+"/content")
	if contentResponse.Code != http.StatusOK || contentResponse.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("image HTTP content response=(%d, %v)", contentResponse.Code, contentResponse.Header())
	}

	vector := "[" + strings.TrimSuffix(strings.Repeat("0,", 768), ",") + "]"
	var postID, suggestionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO posts (tenant_id, title, body, subject, subject_key, subject_confidence, embedding, embedding_model)
		VALUES ($1::uuid, 'Red fox field notes', 'A short article about red foxes.', 'red fox', 'red fox', 0.95, $2::vector, 'integration-test')
		RETURNING id::text`, tenantID, vector).Scan(&postID); err != nil {
		t.Fatal("could not create test post")
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO match_suggestions (tenant_id, post_id, image_id, rank, similarity, accepted)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, 0.91, true) RETURNING id::text`, tenantID, postID, upload.ImageID).Scan(&suggestionID); err != nil {
		t.Fatal("could not create test match suggestion")
	}
	matchService := matching.New(pool, nil, 0.65, 0.35)
	matchRouter := NewRouterWithOperations(testLogger(), pool, time.Second, imageService, matchService, 10<<20)

	emptyHistory, err := matchService.ListReviewHistory(ctx, suggestionID, 20, 0)
	if err != nil || emptyHistory.Total != 0 || len(emptyHistory.Items) != 0 {
		t.Fatalf("new suggestion should have empty review history (total=%d, items=%d, err=%v)", emptyHistory.Total, len(emptyHistory.Items), err)
	}
	emptyHistoryResponse := performRequest(matchRouter, "/v1/matches/"+suggestionID+"/reviews")
	if emptyHistoryResponse.Code != http.StatusOK || !strings.Contains(emptyHistoryResponse.Body.String(), `"total":0`) || !strings.Contains(emptyHistoryResponse.Body.String(), `"items":[]`) {
		t.Fatalf("empty review history HTTP response=(%d, %s)", emptyHistoryResponse.Code, emptyHistoryResponse.Body.String())
	}

	const nonexistentSuggestionID = "99999999-9999-4999-8999-999999999999"
	if _, err := matchService.ListReviewHistory(ctx, nonexistentSuggestionID, 20, 0); !errors.Is(err, matching.ErrSuggestionNotFound) {
		t.Fatalf("nonexistent suggestion history error=%v, want ErrSuggestionNotFound", err)
	}
	nonexistentResponse := performRequest(matchRouter, "/v1/matches/"+nonexistentSuggestionID+"/reviews")
	if nonexistentResponse.Code != http.StatusNotFound {
		t.Fatalf("nonexistent suggestion history status=%d, want 404", nonexistentResponse.Code)
	}

	var otherTenantID, otherImageID, otherPostID, otherSuggestionID string
	if err := pool.QueryRow(ctx, `INSERT INTO tenants (name) VALUES ('integration-test other tenant') RETURNING id::text`).Scan(&otherTenantID); err != nil {
		t.Fatal("could not create the second test tenant")
	}
	defer func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1::uuid`, otherTenantID) }()
	if err := pool.QueryRow(ctx, `
		INSERT INTO images (tenant_id, sha256, storage_key, mime_type, size_bytes, status)
		VALUES ($1::uuid, $2, 'other-tenant-test.png', 'image/png', 1, 'processed') RETURNING id::text`, otherTenantID, strings.Repeat("b", 64)).Scan(&otherImageID); err != nil {
		t.Fatal("could not create second-tenant test image")
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO posts (tenant_id, title, body, subject, subject_key, subject_confidence, embedding, embedding_model)
		VALUES ($1::uuid, 'Other tenant test post', 'Other tenant post body.', 'red fox', 'red fox', 0.95, $2::vector, 'integration-test')
		RETURNING id::text`, otherTenantID, vector).Scan(&otherPostID); err != nil {
		t.Fatal("could not create second-tenant test post")
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO match_suggestions (tenant_id, post_id, image_id, rank, similarity, accepted)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 1, 0.8, true) RETURNING id::text`, otherTenantID, otherPostID, otherImageID).Scan(&otherSuggestionID); err != nil {
		t.Fatal("could not create second-tenant test suggestion")
	}
	if _, err := matchService.ListReviewHistory(ctx, otherSuggestionID, 20, 0); !errors.Is(err, matching.ErrSuggestionNotFound) {
		t.Fatalf("cross-tenant history error=%v, want ErrSuggestionNotFound", err)
	}
	if _, err := matchService.Review(ctx, otherSuggestionID, matching.Review{Decision: "approved"}); !errors.Is(err, matching.ErrSuggestionNotFound) {
		t.Fatalf("cross-tenant review error=%v, want ErrSuggestionNotFound", err)
	}
	otherTenantHistoryResponse := performRequest(matchRouter, "/v1/matches/"+otherSuggestionID+"/reviews")
	if otherTenantHistoryResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant history status=%d, want 404", otherTenantHistoryResponse.Code)
	}
	otherTenantReviewResponse := performJSONRequest(matchRouter, http.MethodPost, "/v1/matches/"+otherSuggestionID+"/review", `{"decision":"approved"}`)
	if otherTenantReviewResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant review status=%d, want 404", otherTenantReviewResponse.Code)
	}

	approval, err := matchService.Review(ctx, suggestionID, matching.Review{Decision: "approved", Comment: "integration approval"})
	if err != nil {
		t.Fatal("could not persist first test review action")
	}
	singleHistory, err := matchService.ListReviewHistory(ctx, suggestionID, 20, 0)
	if err != nil || singleHistory.Total != 1 || len(singleHistory.Items) != 1 || singleHistory.Items[0].ID != approval.ID || singleHistory.Items[0].Decision != "approved" || singleHistory.Items[0].Comment != "integration approval" || singleHistory.Items[0].CreatedAt.IsZero() {
		t.Fatalf("single review history is incorrect (total=%d, items=%d, err=%v)", singleHistory.Total, len(singleHistory.Items), err)
	}
	rejection, err := matchService.Review(ctx, suggestionID, matching.Review{Decision: "rejected", Comment: "integration rejection"})
	if err != nil {
		t.Fatal("could not persist second test review action")
	}
	matchPage, err := matchService.ListMatches(ctx, 20, 0)
	if err != nil || matchPage.Total != 1 || len(matchPage.Items) != 1 || matchPage.Items[0].LatestReview == nil || matchPage.Items[0].LatestReview.ID != rejection.ID || matchPage.Items[0].LatestReview.Decision != "rejected" || matchPage.Items[0].ReviewCount != 2 || matchPage.Items[0].Rank != 1 || matchPage.Items[0].Confidence != 0.94 || matchPage.Items[0].SubjectConfidence != 0.95 || !matchPage.Items[0].Accepted {
		t.Fatalf("recent matches did not return review state (total=%d, err=%v)", matchPage.Total, err)
	}
	reviewHistory, err := matchService.ListReviewHistory(ctx, suggestionID, 20, 0)
	if err != nil || reviewHistory.Total != 2 || len(reviewHistory.Items) != 2 {
		t.Fatalf("review history should preserve both actions (total=%d, items=%d, err=%v)", reviewHistory.Total, len(reviewHistory.Items), err)
	}
	reviewIDs := map[string]matching.ReviewResult{}
	for _, review := range reviewHistory.Items {
		reviewIDs[review.ID] = review
	}
	if reviewIDs[approval.ID].Decision != "approved" || reviewIDs[approval.ID].Comment != "integration approval" || reviewIDs[rejection.ID].Decision != "rejected" || reviewIDs[rejection.ID].Comment != "integration rejection" {
		t.Fatalf("review history lost one of the append-only actions: %+v", reviewIDs)
	}
	if reviewHistory.Items[0].CreatedAt.Before(reviewHistory.Items[1].CreatedAt) {
		t.Fatal("review history is not ordered newest first")
	}
	matchResponse := performRequest(matchRouter, "/v1/matches?limit=20")
	if matchResponse.Code != http.StatusOK || !strings.Contains(matchResponse.Body.String(), `"review_count":2`) || !strings.Contains(matchResponse.Body.String(), `"accepted":true`) {
		t.Fatalf("match HTTP response=(%d, %s)", matchResponse.Code, matchResponse.Body.String())
	}
	historyResponse := performRequest(matchRouter, "/v1/matches/"+suggestionID+"/reviews")
	if historyResponse.Code != http.StatusOK || !strings.Contains(historyResponse.Body.String(), `"comment":"integration approval"`) || !strings.Contains(historyResponse.Body.String(), `"comment":"integration rejection"`) {
		t.Fatalf("review history HTTP response=(%d, %s)", historyResponse.Code, historyResponse.Body.String())
	}

	// The service must reject malformed storage keys even if a database row is changed.
	if _, err := pool.Exec(ctx, `UPDATE images SET storage_key='../outside.png' WHERE id=$1::uuid AND tenant_id=$2::uuid`, upload.ImageID, tenantID); err != nil {
		t.Fatal("could not prepare traversal check")
	}
	if _, err := imageService.Content(ctx, upload.ImageID); err == nil {
		t.Fatal("image content service accepted a path traversal key")
	}
}

func performJSONRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

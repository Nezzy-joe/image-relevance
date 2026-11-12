package evaluation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nezzy-joe/image-relevance/internal/matching"
)

func TestLoadManifestValidatesSubmissionMinimums(t *testing.T) {
	dir := t.TempDir()
	imagesPath := filepath.Join(dir, "images.csv")
	articlesPath := filepath.Join(dir, "articles.csv")
	var images strings.Builder
	images.WriteString("image_id,category,source_url,license\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&images, "image-%02d,category-%d,https://example.test/image/%d,CC BY 4.0\n", i, i%4, i)
	}
	if err := os.WriteFile(imagesPath, []byte(images.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	var articles strings.Builder
	articles.WriteString("post_id,expected_image_ids,negative_image_ids,scenario\n")
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&articles, "post-%02d,image-%02d,image-%02d,positive\n", i, i, i+10)
	}
	articles.WriteString("post-08,image-08,image-20,mismatch_guard\n")
	articles.WriteString("post-09,,image-21,no_suitable\n")
	if err := os.WriteFile(articlesPath, []byte(articles.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(imagesPath, articlesPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Images) != 40 || len(manifest.Articles) != 10 {
		t.Fatalf("manifest counts images=%d articles=%d", len(manifest.Images), len(manifest.Articles))
	}
}

func TestEvaluateReportsMetricsFromResults(t *testing.T) {
	manifest := Manifest{
		Images: []Image{{ID: "fox", Category: "animal"}, {ID: "wolf", Category: "animal"}, {ID: "car", Category: "vehicle"}, {ID: "bird", Category: "animal"}},
		Articles: []Article{
			{PostID: "a1", Scenario: "positive", Expected: []string{"fox"}},
			{PostID: "a2", Scenario: "mismatch_guard", Expected: []string{"car"}, Negative: []string{"wolf"}},
			{PostID: "a3", Scenario: "no_suitable", Negative: []string{"bird"}},
		},
	}
	results := map[string]matching.MatchResult{
		"a1": {Status: "matched", Suggestions: []matching.Candidate{{ImageID: "fox", Accepted: true}}},
		"a2": {Status: "matched", Suggestions: []matching.Candidate{{ImageID: "wolf", Accepted: true}, {ImageID: "car", Accepted: true}}},
		"a3": {Status: "no_confident_match", Suggestions: []matching.Candidate{}},
	}
	report, err := Evaluate(context.Background(), manifest, func(_ context.Context, postID string, _ []string) (matching.MatchResult, error) {
		return results[postID], nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Top1Precision != 0.5 || report.RecommendationCoverage != 2.0/3 || report.AcceptedRecommendationPrec != 2.0/3 || report.IncorrectAcceptedMatches != 1 || report.Abstentions != 1 {
		t.Fatalf("unexpected evaluation metrics: %+v", report)
	}
}

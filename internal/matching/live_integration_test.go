package matching

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/config"
	"github.com/Nezzy-joe/image-relevance/internal/database"
	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/providers/gemini"
)

func TestLiveCrossModalCosineRankingAndMismatchGuard(t *testing.T) {
	if os.Getenv("GEMINI_LIVE_TEST") != "1" {
		t.Skip("set GEMINI_LIVE_TEST=1 to run live Gemini and PostgreSQL matching checks")
	}
	imagePath := os.Getenv("GEMINI_TEST_IMAGE_PATH")
	if imagePath == "" {
		t.Skip("set GEMINI_TEST_IMAGE_PATH to a real JPEG image")
	}
	image, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatal("could not read the configured image fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("configuration failed")
	}
	provider, err := gemini.New(ctx, cfg)
	if err != nil {
		t.Fatal("Gemini client initialization failed")
	}
	imageVector, imageUsage, err := provider.EmbedImage(ctx, image, "image/jpeg")
	if err != nil {
		t.Fatalf("real image embedding failed (error_type=%T)", err)
	}
	if !imageUsage.Success || len(imageVector) != gemini.EmbeddingDimensions {
		t.Fatal("image embedding returned an invalid vector")
	}
	articles := []struct {
		name, subject, body string
	}{
		{"red_fox", "red fox", "The red fox (Vulpes vulpes) is a widespread fox known for its russet coat, bushy tail, forest habitat, and varied diet."},
		{"gray_wolf", "gray wolf", "The gray wolf (Canis lupus) is a large wild canine that lives in packs and inhabits forests, tundra, and grasslands."},
		{"bicycle_maintenance", "bicycle maintenance", "A practical guide to replacing a bicycle chain, adjusting gears, and maintaining a bicycle drivetrain."},
	}
	type rankedArticle struct {
		name, subject, key string
		vector             string
		similarity         float64
		accepted           bool
		reason             string
	}
	ranked := make([]rankedArticle, 0, len(articles))
	for _, article := range articles {
		vector, usage, err := provider.EmbedText(ctx, article.body)
		if err != nil {
			t.Fatalf("text embedding for %s failed (error_type=%T)", article.name, err)
		}
		if !usage.Success || len(vector) != gemini.EmbeddingDimensions {
			t.Fatalf("text embedding for %s returned an invalid vector", article.name)
		}
		literal, err := formatVector(vector)
		if err != nil {
			t.Fatalf("text vector for %s is invalid", article.name)
		}
		ranked = append(ranked, rankedArticle{name: article.name, subject: article.subject, key: domain.SubjectKey(article.subject), vector: literal})
	}
	imageLiteral, err := formatVector(imageVector)
	if err != nil {
		t.Fatal("image vector is invalid")
	}
	pool, err := database.OpenPool(ctx, database.PoolConfig{URL: cfg.DatabaseURL, MaxConns: cfg.DatabaseMaxConns})
	if err != nil {
		t.Fatal("PostgreSQL connection failed")
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `
		SELECT title, 1 - (embedding <=> $1::vector) AS similarity
		FROM (VALUES ('red_fox', $2::vector), ('gray_wolf', $3::vector), ('bicycle_maintenance', $4::vector)) AS candidates(title, embedding)
		ORDER BY embedding <=> $1::vector`, imageLiteral, ranked[0].vector, ranked[1].vector, ranked[2].vector)
	if err != nil {
		t.Fatal("pgvector cosine ranking query failed")
	}
	positions := map[string]float64{}
	var order []string
	for rows.Next() {
		var name string
		var similarity float64
		if err := rows.Scan(&name, &similarity); err != nil {
			rows.Close()
			t.Fatal("could not read pgvector ranking")
		}
		positions[name] = similarity
		order = append(order, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal("pgvector ranking query failed")
	}
	if len(order) != len(articles) {
		t.Fatalf("pgvector returned %d candidates; want %d", len(order), len(articles))
	}
	sort.Slice(ranked, func(i, j int) bool { return positions[ranked[i].name] > positions[ranked[j].name] })
	for i := range ranked {
		ranked[i].similarity = positions[ranked[i].name]
		ranked[i].accepted, ranked[i].reason = candidateDecision(ranked[i].subject, ranked[i].key, "red fox", "red fox", 0.96, cfg.ConfidenceFloor, ranked[i].similarity, cfg.SimilarityFloor, false)
		t.Logf("candidate=%s cosine=%.4f decision=%s", ranked[i].name, ranked[i].similarity, map[bool]string{true: "accepted", false: "rejected"}[ranked[i].accepted])
	}
	if ranked[0].name != "red_fox" {
		t.Fatalf("red fox was not the top cosine-ranked article; ranking=%v", order)
	}
	for _, candidate := range ranked {
		switch candidate.name {
		case "red_fox":
			if !candidate.accepted {
				t.Fatalf("red fox article was not accepted: %s", candidate.reason)
			}
		case "gray_wolf", "bicycle_maintenance":
			if candidate.accepted {
				t.Fatalf("mismatched candidate %s was accepted", candidate.name)
			}
		}
	}
}

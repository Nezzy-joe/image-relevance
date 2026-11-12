package gemini

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/config"
	"google.golang.org/genai"
)

// Run explicitly with GEMINI_LIVE_TEST=1 after configuring GEMINI_API_KEY.
func TestLiveGeminiGeneration(t *testing.T) {
	if os.Getenv("GEMINI_LIVE_TEST") != "1" {
		t.Skip("set GEMINI_LIVE_TEST=1 to call real Gemini generation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := liveClient(t, ctx)
	resolved, generationUsage, err := client.ResolveSubject(ctx, "Connectivity check", "A short diagnostic request used only to verify model access.")
	if err != nil {
		var apiError genai.APIError
		if errors.As(err, &apiError) {
			t.Fatalf("Gemini text generation returned HTTP %d status=%s", apiError.Code, apiError.Status)
		}
		safeError := "other"
		switch err.Error() {
		case "subject JSON has an invalid size", "subject is not valid structured JSON", "subject must contain exactly one JSON object", "subject confidence is missing", "subject must contain 1 to 120 characters", "subject confidence must be a finite number between 0 and 1":
			safeError = err.Error()
		}
		t.Fatalf("Gemini text generation request failed (error_type=%T safe_error=%q)", err, safeError)
	}
	if resolved.Subject == "" || !generationUsage.Success {
		t.Fatal("Gemini text generation returned no valid structured result")
	}
	t.Logf("Gemini generation succeeded: model=%s", generationUsage.Model)
}

func TestLiveGeminiImageClassification(t *testing.T) {
	if os.Getenv("GEMINI_LIVE_TEST") != "1" {
		t.Skip("set GEMINI_LIVE_TEST=1 to call real Gemini image classification")
	}
	imagePath := os.Getenv("GEMINI_TEST_IMAGE_PATH")
	if imagePath == "" {
		t.Skip("set GEMINI_TEST_IMAGE_PATH to a real JPEG image")
	}
	image, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatal("could not read the configured image fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := liveClient(t, ctx)
	metadata, usage, err := client.AnalyzeImage(ctx, image, "image/jpeg")
	if err != nil {
		var apiError genai.APIError
		if errors.As(err, &apiError) {
			t.Fatalf("Gemini image classification returned HTTP %d status=%s", apiError.Code, apiError.Status)
		}
		t.Fatalf("Gemini image classification failed (error_type=%T)", err)
	}
	if err := metadata.Validate(); err != nil {
		t.Fatalf("Gemini image classification returned invalid metadata: %v", err)
	}
	if !usage.Success {
		t.Fatal("Gemini image classification usage was not marked successful")
	}
	t.Logf("Gemini classification succeeded: model=%s subject=%q category=%q confidence=%.2f", usage.Model, metadata.Subject, metadata.Category, metadata.Confidence)
}

func TestLiveGeminiEmbedding(t *testing.T) {
	if os.Getenv("GEMINI_LIVE_TEST") != "1" {
		t.Skip("set GEMINI_LIVE_TEST=1 to call real Gemini embedding")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := liveClient(t, ctx)
	embedding, embeddingUsage, err := client.EmbedText(ctx, "Gemini embedding connectivity check")
	if err != nil {
		var apiError genai.APIError
		if errors.As(err, &apiError) {
			t.Fatalf("Gemini text embedding returned HTTP %d status=%s", apiError.Code, apiError.Status)
		}
		t.Fatalf("Gemini embedding request failed (error_type=%T)", err)
	}
	if len(embedding) != EmbeddingDimensions || !embeddingUsage.Success {
		t.Fatalf("embedding check returned %d dimensions; want %d", len(embedding), EmbeddingDimensions)
	}
	t.Logf("Gemini embedding succeeded: model=%s dimensions=%d", embeddingUsage.Model, len(embedding))
}

func TestLiveGeminiImageEmbedding(t *testing.T) {
	if os.Getenv("GEMINI_LIVE_TEST") != "1" {
		t.Skip("set GEMINI_LIVE_TEST=1 to call real Gemini image embedding")
	}
	imagePath := os.Getenv("GEMINI_TEST_IMAGE_PATH")
	if imagePath == "" {
		t.Skip("set GEMINI_TEST_IMAGE_PATH to a real JPEG or PNG image")
	}
	image, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatal("could not read the configured image fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client := liveClient(t, ctx)
	embedding, usage, err := client.EmbedImage(ctx, image, "image/jpeg")
	if err != nil {
		var apiError genai.APIError
		if errors.As(err, &apiError) {
			t.Fatalf("Gemini image embedding returned HTTP %d status=%s", apiError.Code, apiError.Status)
		}
		t.Fatalf("Gemini image embedding request failed (error_type=%T)", err)
	}
	if len(embedding) != EmbeddingDimensions || !usage.Success {
		t.Fatalf("image embedding check returned %d dimensions; want %d", len(embedding), EmbeddingDimensions)
	}
	t.Logf("Gemini image embedding succeeded: model=%s dimensions=%d", usage.Model, len(embedding))
}

func liveClient(t *testing.T, ctx context.Context) *Client {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal("configuration failed")
	}
	client, err := New(ctx, cfg)
	if err != nil {
		t.Fatal("Gemini client initialization failed")
	}
	return client
}

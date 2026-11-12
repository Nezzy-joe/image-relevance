package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/genai"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestEmbedImageSendsImageAndRequests768Dimensions(t *testing.T) {
	image := []byte{0xff, 0xd8, 0xff, 0x01}
	values := make([]float32, EmbeddingDimensions)
	for i := range values {
		values[i] = 0.25
	}
	responseBody, err := json.Marshal(map[string]any{
		"embeddings": []any{map[string]any{
			"values":     values,
			"statistics": map[string]any{"tokenCount": 1},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  "unit-test-key",
		Backend: genai.BackendGeminiAPI,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if !strings.Contains(request.URL.Path, "gemini-embedding-2:batchEmbedContents") {
				t.Errorf("unexpected endpoint %q", request.URL.Path)
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatal(err)
			}
			requests, ok := payload["requests"].([]any)
			if !ok || len(requests) != 1 {
				t.Fatalf("expected one embedding request, got %T", payload["requests"])
			}
			requestBody, ok := requests[0].(map[string]any)
			if !ok {
				t.Fatalf("embedding request has unexpected type %T", requests[0])
			}
			if got, _ := requestBody["outputDimensionality"].(float64); got != EmbeddingDimensions {
				t.Errorf("outputDimensionality=%v, want %d", requestBody["outputDimensionality"], EmbeddingDimensions)
			}
			content, ok := requestBody["content"].(map[string]any)
			if !ok {
				t.Fatalf("image embedding content has unexpected type %T", requestBody["content"])
			}
			parts, ok := content["parts"].([]any)
			if !ok || len(parts) != 1 {
				t.Fatalf("expected one inline image part, got %T", content["parts"])
			}
			part, ok := parts[0].(map[string]any)
			if !ok {
				t.Fatalf("image part has unexpected type %T", parts[0])
			}
			inline, ok := part["inlineData"].(map[string]any)
			if !ok || inline["mimeType"] != "image/jpeg" || inline["data"] != base64.StdEncoding.EncodeToString(image) {
				t.Errorf("inline image payload did not match JPEG input")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(responseBody)),
				Request:    request,
			}, nil
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	provider := &Client{client: client, embeddingModel: "gemini-embedding-2"}
	embedding, usage, err := provider.EmbedImage(context.Background(), image, "image/jpeg")
	if err != nil {
		t.Fatalf("EmbedImage() error = %v", err)
	}
	if len(embedding) != EmbeddingDimensions || !usage.Success || usage.Operation != "image_embedding" {
		t.Fatalf("EmbedImage() returned dimensions=%d success=%v operation=%q", len(embedding), usage.Success, usage.Operation)
	}
	if usage.CostUSD == nil || *usage.CostUSD != 0.00012 {
		t.Fatalf("image embedding cost estimate = %v, want $0.00012", usage.CostUSD)
	}
}

func TestEmbedImageRejectsEmptyOrUnsupportedInput(t *testing.T) {
	provider := &Client{embeddingModel: "gemini-embedding-2"}
	for _, test := range []struct {
		name string
		data []byte
		mime string
	}{
		{name: "empty", mime: "image/jpeg"},
		{name: "unsupported MIME", data: []byte("image"), mime: "image/gif"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, usage, err := provider.EmbedImage(context.Background(), test.data, test.mime)
			if err == nil || usage.ErrorCode != "invalid_image_embedding_input" {
				t.Fatalf("EmbedImage() error=%v error_code=%q", err, usage.ErrorCode)
			}
		})
	}
}

package gemini

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/config"
	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"google.golang.org/genai"
)

const EmbeddingDimensions = 768

type Usage struct {
	Provider     string
	Model        string
	Operation    string
	InputTokens  *int64
	OutputTokens *int64
	Latency      time.Duration
	Success      bool
	ErrorCode    string
	CostUSD      *float64
	CostBasis    string
}

type Client struct {
	client         *genai.Client
	visionModel    string
	embeddingModel string
}

func New(ctx context.Context, cfg config.Config) (*Client, error) {
	if strings.TrimSpace(cfg.GeminiAPIKey) == "" {
		return nil, errors.New("GEMINI_API_KEY is required by the Gemini worker")
	}
	requestTimeout := cfg.GeminiRequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 25 * time.Second
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:      cfg.GeminiAPIKey,
		Backend:     genai.BackendGeminiAPI,
		HTTPOptions: genai.HTTPOptions{Timeout: &requestTimeout},
	})
	if err != nil {
		return nil, errors.New("Gemini client initialization failed")
	}
	return &Client{client: client, visionModel: cfg.VisionModel, embeddingModel: cfg.EmbeddingModel}, nil
}

func (c *Client) AnalyzeImage(ctx context.Context, image []byte, mimeType string) (domain.ImageMetadata, Usage, error) {
	usage := newUsage(c.visionModel, "image_classification")
	started := time.Now()
	contents := []*genai.Content{{Role: "user", Parts: []*genai.Part{
		{Text: "Analyze the image and return only the requested JSON. Identify the most specific visible subject, a broad category, observable attributes, and a concise factual caption. Confidence must represent your uncertainty; do not invent details. If uncertain, lower confidence."},
		{InlineData: &genai.Blob{Data: image, MIMEType: mimeType}},
	}}}
	response, err := c.client.Models.GenerateContent(ctx, c.visionModel, contents, imageMetadataConfig())
	usage.Latency = time.Since(started)
	if err != nil {
		usage.ErrorCode = "provider_request_failed"
		applyEstimatedCost(&usage, time.Now())
		return domain.ImageMetadata{}, usage, err
	}
	if response == nil {
		usage.ErrorCode = "empty_provider_response"
		applyEstimatedCost(&usage, time.Now())
		return domain.ImageMetadata{}, usage, errors.New("Gemini returned an empty response")
	}
	if response.UsageMetadata != nil {
		usage.InputTokens = int64Pointer(int64(response.UsageMetadata.PromptTokenCount))
		usage.OutputTokens = int64Pointer(int64(response.UsageMetadata.CandidatesTokenCount + response.UsageMetadata.ThoughtsTokenCount))
	}
	metadata, err := domain.ParseImageMetadata([]byte(response.Text()))
	if err != nil {
		usage.ErrorCode = "invalid_structured_response"
		applyEstimatedCost(&usage, time.Now())
		return domain.ImageMetadata{}, usage, err
	}
	usage.Success = true
	applyEstimatedCost(&usage, time.Now())
	return metadata, usage, nil
}

func (c *Client) ResolveSubject(ctx context.Context, title, body string) (domain.SubjectResolution, Usage, error) {
	usage := newUsage(c.visionModel, "subject_resolution")
	started := time.Now()
	content := fmt.Sprintf("Extract the article's primary specific subject as a short noun phrase. Do not add a subject that is not supported by the title/body. Return low confidence if multiple subjects are equally central.\nTitle: %s\nBody: %s", title, body)
	response, err := c.client.Models.GenerateContent(ctx, c.visionModel, genai.Text(content), subjectConfig())
	usage.Latency = time.Since(started)
	if err != nil {
		usage.ErrorCode = "provider_request_failed"
		applyEstimatedCost(&usage, time.Now())
		return domain.SubjectResolution{}, usage, err
	}
	if response == nil {
		usage.ErrorCode = "empty_provider_response"
		applyEstimatedCost(&usage, time.Now())
		return domain.SubjectResolution{}, usage, errors.New("Gemini returned an empty response")
	}
	if response.UsageMetadata != nil {
		usage.InputTokens = int64Pointer(int64(response.UsageMetadata.PromptTokenCount))
		usage.OutputTokens = int64Pointer(int64(response.UsageMetadata.CandidatesTokenCount + response.UsageMetadata.ThoughtsTokenCount))
	}
	resolved, err := domain.ParseSubjectResolution([]byte(response.Text()))
	if err != nil {
		usage.ErrorCode = "invalid_structured_response"
		applyEstimatedCost(&usage, time.Now())
		return domain.SubjectResolution{}, usage, err
	}
	usage.Success = true
	applyEstimatedCost(&usage, time.Now())
	return resolved, usage, nil
}

func (c *Client) EmbedText(ctx context.Context, text string) ([]float32, Usage, error) {
	return c.embed(ctx, "text_embedding", genai.Text(text))
}

func (c *Client) EmbedImage(ctx context.Context, image []byte, mimeType string) ([]float32, Usage, error) {
	usage := newUsage(c.embeddingModel, "image_embedding")
	if len(image) == 0 || (mimeType != "image/jpeg" && mimeType != "image/png") {
		usage.ErrorCode = "invalid_image_embedding_input"
		return nil, usage, errors.New("image embedding requires non-empty JPEG or PNG data")
	}
	content := []*genai.Content{genai.NewContentFromParts([]*genai.Part{genai.NewPartFromBytes(image, mimeType)}, genai.RoleUser)}
	return c.embed(ctx, usage.Operation, content)
}

func (c *Client) embed(ctx context.Context, operation string, contents []*genai.Content) ([]float32, Usage, error) {
	usage := newUsage(c.embeddingModel, operation)
	started := time.Now()
	response, err := c.client.Models.EmbedContent(ctx, c.embeddingModel, contents, &genai.EmbedContentConfig{
		OutputDimensionality: genai.Ptr[int32](EmbeddingDimensions),
	})
	usage.Latency = time.Since(started)
	if err != nil {
		usage.ErrorCode = "provider_request_failed"
		applyEstimatedCost(&usage, time.Now())
		return nil, usage, err
	}
	if response == nil || len(response.Embeddings) != 1 || response.Embeddings[0] == nil {
		usage.ErrorCode = "invalid_embedding_response"
		applyEstimatedCost(&usage, time.Now())
		return nil, usage, errors.New("Gemini returned no embedding")
	}
	values := response.Embeddings[0].Values
	if len(values) != EmbeddingDimensions {
		usage.ErrorCode = "invalid_embedding_dimensions"
		applyEstimatedCost(&usage, time.Now())
		return nil, usage, fmt.Errorf("embedding dimension mismatch: got %d, want %d", len(values), EmbeddingDimensions)
	}
	for _, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			usage.ErrorCode = "invalid_embedding_values"
			applyEstimatedCost(&usage, time.Now())
			return nil, usage, errors.New("Gemini returned non-finite embedding values")
		}
	}
	if stats := response.Embeddings[0].Statistics; stats != nil && stats.TokenCount >= 0 {
		usage.InputTokens = int64Pointer(int64(stats.TokenCount))
	}
	usage.Success = true
	applyEstimatedCost(&usage, time.Now())
	return values, usage, nil
}

func newUsage(model, operation string) Usage {
	return Usage{
		Provider:  "gemini",
		Model:     model,
		Operation: operation,
		CostBasis: "no estimated cost: usage is unavailable or no verified model rate is configured",
	}
}

func int64Pointer(value int64) *int64 { return &value }

func applyEstimatedCost(usage *Usage, at time.Time) {
	if usage.Model == "gemini-embedding-2" && usage.Operation == "image_embedding" {
		if !usage.Success {
			usage.CostBasis = "no estimate: image embedding request did not succeed"
			return
		}
		cost := 0.00012
		usage.CostUSD = &cost
		usage.CostBasis = "estimated standard paid Gemini Embedding 2 image rate per image; excludes free tier, credits, and discounts; Google pricing page checked 2026-11-12"
		return
	}
	if usage.InputTokens == nil {
		usage.CostBasis = "no estimate: provider input token usage was not returned"
		return
	}
	var inputRate, outputRate float64
	switch {
	case usage.Model == "gemini-3.1-flash-lite":
		inputRate, outputRate = 0.25, 1.50
		usage.CostBasis = "estimated Gemini 3.1 Flash-Lite Standard paid list rate per 1M input/output tokens; excludes free tier, credits, and discounts; Google pricing page checked 2026-11-12"
	case usage.Model == "gemini-3.8-flash":
		inputRate, outputRate = 0.75, 3.75
		if !at.Before(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
			inputRate, outputRate = 1.50, 7.50
		}
		usage.CostBasis = "estimated standard paid list rate per 1M tokens; excludes free tier, credits, and discounts; Google pricing page checked 2026-11-11"
	case usage.Model == "gemini-embedding-2" && usage.Operation == "text_embedding":
		inputRate = 0.20
		usage.CostBasis = "estimated Gemini Embedding 2 Standard paid text rate per 1M input tokens; excludes free tier, credits, and discounts; Google pricing page checked 2026-11-12"
	default:
		usage.CostBasis = "no estimate: no verified rate configured for this model and operation"
		return
	}
	cost := float64(*usage.InputTokens) * inputRate / 1_000_000
	if usage.OutputTokens != nil {
		cost += float64(*usage.OutputTokens) * outputRate / 1_000_000
	}
	usage.CostUSD = &cost
}

func imageMetadataConfig() *genai.GenerateContentConfig {
	return &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"subject":    {Type: genai.TypeString},
				"category":   {Type: genai.TypeString},
				"attributes": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
				"caption":    {Type: genai.TypeString},
				"confidence": {Type: genai.TypeNumber, Minimum: genai.Ptr[float64](0), Maximum: genai.Ptr[float64](1)},
			},
			Required: []string{"subject", "category", "attributes", "caption", "confidence"},
		},
		ThinkingConfig:  &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow},
		MaxOutputTokens: 512,
	}
}

func subjectConfig() *genai.GenerateContentConfig {
	return &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"subject":    {Type: genai.TypeString},
				"confidence": {Type: genai.TypeNumber, Minimum: genai.Ptr[float64](0), Maximum: genai.Ptr[float64](1)},
			},
			Required: []string{"subject", "confidence"},
		},
		ThinkingConfig:  &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow},
		MaxOutputTokens: 512,
	}
}

package matching

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/providers/gemini"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const devTenantName = "image-relevance development tenant"

var ErrPostNotFound = errors.New("post not found")
var ErrSuggestionNotFound = errors.New("match suggestion not found")
var ErrInvalidReview = errors.New("review decision must be approved or rejected")
var ErrReviewTooLong = errors.New("review comment exceeds 2000 characters")
var ErrInvalidPost = errors.New("title and body are required and must fit the API limits")

type Provider interface {
	ResolveSubject(context.Context, string, string) (domain.SubjectResolution, gemini.Usage, error)
	EmbedText(context.Context, string) ([]float32, gemini.Usage, error)
}

type Service struct {
	pool            *pgxpool.Pool
	provider        Provider
	confidenceFloor float64
	similarityFloor float64
	limit           int
}

func New(pool *pgxpool.Pool, provider Provider, confidenceFloor, similarityFloor float64) *Service {
	return &Service{pool: pool, provider: provider, confidenceFloor: confidenceFloor, similarityFloor: similarityFloor, limit: 20}
}

type PostInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type Post struct {
	ID                string    `json:"id"`
	Title             string    `json:"title"`
	Subject           string    `json:"subject"`
	SubjectConfidence float64   `json:"subject_confidence"`
	CreatedAt         time.Time `json:"created_at"`
}

func (s *Service) CreatePost(ctx context.Context, input PostInput) (Post, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Body = strings.TrimSpace(input.Body)
	if len(input.Title) == 0 || len(input.Title) > 200 || len(input.Body) == 0 || len(input.Body) > 20_000 {
		return Post{}, ErrInvalidPost
	}
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return Post{}, err
	}
	resolved, subjectUsage, err := s.provider.ResolveSubject(ctx, input.Title, input.Body)
	if err != nil {
		if trackErr := s.trackUsage(ctx, tenantID, "", subjectUsage); trackErr != nil {
			return Post{}, errors.New("could not record subject-resolution usage")
		}
		return Post{}, errors.New("subject resolution failed")
	}
	if resolved.Subject == "" || resolved.Confidence < 0 || resolved.Confidence > 1 || math.IsNaN(resolved.Confidence) || math.IsInf(resolved.Confidence, 0) {
		if trackErr := s.trackUsage(ctx, tenantID, "", subjectUsage); trackErr != nil {
			return Post{}, errors.New("could not record subject-resolution usage")
		}
		return Post{}, errors.New("subject resolution returned invalid structured data")
	}
	embeddingText := input.Title + "\n\n" + input.Body
	embedding, embeddingUsage, err := s.provider.EmbedText(ctx, embeddingText)
	if err != nil {
		if trackErr := s.recordUnattachedUsage(ctx, tenantID, subjectUsage, embeddingUsage); trackErr != nil {
			return Post{}, errors.New("could not record post provider usage")
		}
		return Post{}, errors.New("post embedding failed")
	}
	if len(embedding) != gemini.EmbeddingDimensions {
		if trackErr := s.recordUnattachedUsage(ctx, tenantID, subjectUsage, embeddingUsage); trackErr != nil {
			return Post{}, errors.New("could not record post provider usage")
		}
		return Post{}, errors.New("post embedding has an unsupported dimension")
	}
	vector, err := formatVector(embedding)
	if err != nil {
		if trackErr := s.recordUnattachedUsage(ctx, tenantID, subjectUsage, embeddingUsage); trackErr != nil {
			return Post{}, errors.New("could not record post provider usage")
		}
		return Post{}, errors.New("post embedding contains invalid values")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		if trackErr := s.recordUnattachedUsage(ctx, tenantID, subjectUsage, embeddingUsage); trackErr != nil {
			return Post{}, errors.New("could not record post provider usage")
		}
		return Post{}, errors.New("could not save post")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var post Post
	err = tx.QueryRow(ctx, `
		INSERT INTO posts (tenant_id, title, body, subject, subject_key, subject_confidence, embedding, embedding_model)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7::vector, $8)
		RETURNING id::text, title, subject, created_at`,
		tenantID, input.Title, input.Body, resolved.Subject, domain.SubjectKey(resolved.Subject), resolved.Confidence, vector, embeddingUsage.Model).
		Scan(&post.ID, &post.Title, &post.Subject, &post.CreatedAt)
	if err != nil {
		if trackErr := s.recordUnattachedUsage(ctx, tenantID, subjectUsage, embeddingUsage); trackErr != nil {
			return Post{}, errors.New("could not record post provider usage")
		}
		return Post{}, errors.New("could not save post")
	}
	post.SubjectConfidence = resolved.Confidence
	if err := insertUsage(ctx, tx, tenantID, post.ID, subjectUsage); err != nil {
		return Post{}, errors.New("could not record subject-resolution usage")
	}
	if err := insertUsage(ctx, tx, tenantID, post.ID, embeddingUsage); err != nil {
		return Post{}, errors.New("could not record embedding usage")
	}
	if err := tx.Commit(ctx); err != nil {
		return Post{}, errors.New("could not commit post and provider usage")
	}
	return post, nil
}

type Candidate struct {
	ImageID         string  `json:"image_id"`
	Subject         string  `json:"subject"`
	Category        string  `json:"category"`
	Similarity      float64 `json:"similarity"`
	Confidence      float64 `json:"confidence"`
	Uncertain       bool    `json:"uncertain"`
	Rank            int     `json:"rank"`
	Accepted        bool    `json:"accepted"`
	RejectionReason string  `json:"rejection_reason,omitempty"`
	SuggestionID    string  `json:"suggestion_id,omitempty"`
}

type MatchResult struct {
	Status      string      `json:"status"`
	Suggestions []Candidate `json:"suggestions"`
	Rejected    []Candidate `json:"rejected,omitempty"`
	Reasons     []string    `json:"reasons"`
}

func (s *Service) Match(ctx context.Context, postID string) (MatchResult, error) {
	return s.MatchCandidates(ctx, postID, nil)
}

func (s *Service) MatchCandidates(ctx context.Context, postID string, imageIDs []string) (MatchResult, error) {
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return MatchResult{}, err
	}
	var title, postSubject, postKey string
	var confidence float64
	err = s.pool.QueryRow(ctx, `SELECT title, subject, subject_key, subject_confidence FROM posts WHERE id=$1::uuid AND tenant_id=$2::uuid`, postID, tenantID).
		Scan(&title, &postSubject, &postKey, &confidence)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchResult{}, ErrPostNotFound
	}
	if err != nil {
		return MatchResult{}, errors.New("could not retrieve post for matching")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT i.id::text, a.subject, a.category, a.subject_key, a.confidence, a.uncertain,
			1 - (e.embedding <=> p.embedding) AS similarity
		FROM posts p
		JOIN image_embeddings e ON e.tenant_id=p.tenant_id
		JOIN images i ON i.id=e.image_id AND i.tenant_id=p.tenant_id AND i.status IN ('processed','needs_review')
		JOIN image_analysis a ON a.image_id=i.id
		WHERE p.id=$1::uuid AND p.tenant_id=$2::uuid AND ($4::uuid[] IS NULL OR i.id=ANY($4::uuid[]))
		ORDER BY e.embedding <=> p.embedding
		LIMIT $3`, postID, tenantID, s.limit, imageIDs)
	if err != nil {
		return MatchResult{}, errors.New("candidate retrieval failed")
	}
	type candidateRow struct {
		id, subject, category, subjectKey string
		confidence, similarity            float64
		uncertain                         bool
	}
	var candidates []candidateRow
	for rows.Next() {
		var candidate candidateRow
		if err := rows.Scan(&candidate.id, &candidate.subject, &candidate.category, &candidate.subjectKey, &candidate.confidence, &candidate.uncertain, &candidate.similarity); err != nil {
			rows.Close()
			return MatchResult{}, errors.New("candidate retrieval failed")
		}
		candidates = append(candidates, candidate)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return MatchResult{}, errors.New("candidate retrieval failed")
	}

	result := MatchResult{Status: "matched", Suggestions: []Candidate{}, Rejected: []Candidate{}, Reasons: []string{}}
	if len(candidates) == 0 {
		result.Status = "no_confident_match"
		result.Reasons = append(result.Reasons, "No candidate satisfied the subject and similarity requirements.")
		return result, nil
	}
	acceptedRank := 0
	for index, item := range candidates {
		candidate := Candidate{ImageID: item.id, Subject: item.subject, Category: item.category, Similarity: item.similarity, Confidence: item.confidence, Uncertain: item.uncertain, Rank: index + 1}
		candidate.Accepted, candidate.RejectionReason = candidateDecision(postSubject, postKey, item.subject, item.subjectKey,
			confidence, s.confidenceFloor, item.similarity, s.similarityFloor, item.uncertain)
		if candidate.Accepted {
			acceptedRank++
			candidate.Rank = acceptedRank
		}
		if candidate.Accepted {
			result.Suggestions = append(result.Suggestions, candidate)
		} else {
			result.Rejected = append(result.Rejected, candidate)
		}
	}
	if len(result.Suggestions) == 0 {
		result.Status = "no_confident_match"
		result.Suggestions = []Candidate{}
		result.Reasons = append(result.Reasons, "No candidate satisfied the subject and similarity requirements.")
	}
	if err := s.saveSuggestions(ctx, tenantID, postID, result); err != nil {
		return MatchResult{}, err
	}
	return result, nil
}

func candidateDecision(articleSubject, articleKey, imageSubject, imageKey string, articleConfidence, confidenceFloor, similarity, similarityFloor float64, uncertain bool) (bool, string) {
	switch {
	case !subjectCompatible(articleKey, imageKey):
		return false, fmt.Sprintf("Subject mismatch: article subject '%s' does not match image subject '%s'.", articleSubject, imageSubject)
	case uncertain:
		return false, "Image classification confidence is low; this image needs human review."
	case articleConfidence < confidenceFloor:
		return false, "The article subject resolution confidence is below the configured threshold."
	case similarity < similarityFloor:
		return false, "The image similarity is below the configured threshold."
	default:
		return true, ""
	}
}

func subjectCompatible(articleKey, imageKey string) bool {
	if articleKey == "" || imageKey == "" {
		return false
	}
	articlePhrase := " " + normalizeSpeciesPlural(articleKey) + " "
	imagePhrase := " " + normalizeSpeciesPlural(imageKey) + " "
	return strings.Contains(articlePhrase, imagePhrase) || strings.Contains(imagePhrase, articlePhrase)
}

func normalizeSpeciesPlural(subject string) string {
	words := strings.Fields(subject)
	for i, word := range words {
		switch word {
		case "wolves":
			words[i] = "wolf"
		case "foxes":
			words[i] = "fox"
		case "eagles":
			words[i] = "eagle"
		case "cheetahs":
			words[i] = "cheetah"
		}
	}
	return strings.Join(words, " ")
}

func (s *Service) saveSuggestions(ctx context.Context, tenantID, postID string, result MatchResult) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return errors.New("could not persist match decisions")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	all := append(append([]Candidate{}, result.Suggestions...), result.Rejected...)
	for i := range all {
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO match_suggestions (tenant_id, post_id, image_id, rank, similarity, accepted, rejection_reason)
			VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, NULLIF($7,'')) RETURNING id::text`,
			tenantID, postID, all[i].ImageID, all[i].Rank, all[i].Similarity, all[i].Accepted, all[i].RejectionReason).Scan(&id)
		if err != nil {
			return errors.New("could not persist match decisions")
		}
		all[i].SuggestionID = id
	}
	if err := tx.Commit(ctx); err != nil {
		return errors.New("could not persist match decisions")
	}
	for i := range result.Suggestions {
		result.Suggestions[i].SuggestionID = all[i].SuggestionID
	}
	for i := range result.Rejected {
		result.Rejected[i].SuggestionID = all[len(result.Suggestions)+i].SuggestionID
	}
	return nil
}

type Review struct {
	Decision string `json:"decision"`
	Comment  string `json:"comment,omitempty"`
}

type ReviewResult struct {
	ID        string    `json:"id"`
	Decision  string    `json:"decision"`
	Comment   string    `json:"comment,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Service) Review(ctx context.Context, suggestionID string, review Review) (ReviewResult, error) {
	if review.Decision != "approved" && review.Decision != "rejected" {
		return ReviewResult{}, ErrInvalidReview
	}
	if len(review.Comment) > 2000 {
		return ReviewResult{}, ErrReviewTooLong
	}
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return ReviewResult{}, err
	}
	var result ReviewResult
	err = s.pool.QueryRow(ctx, `
		INSERT INTO review_actions (tenant_id, suggestion_id, decision, comment)
		SELECT $1::uuid, ms.id, $3, NULLIF($4,'') FROM match_suggestions ms
		WHERE ms.id=$2::uuid AND ms.tenant_id=$1::uuid
		RETURNING id::text, decision, COALESCE(comment,''), created_at`, tenantID, suggestionID, review.Decision, strings.TrimSpace(review.Comment)).
		Scan(&result.ID, &result.Decision, &result.Comment, &result.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReviewResult{}, ErrSuggestionNotFound
	}
	if err != nil {
		return ReviewResult{}, errors.New("could not save review action")
	}
	return result, nil
}

func (s *Service) tenantID(ctx context.Context) (string, error) {
	var tenantID string
	err := s.pool.QueryRow(ctx, `SELECT id::text FROM tenants WHERE name=$1 AND status='active' ORDER BY created_at LIMIT 1`, devTenantName).Scan(&tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNoTenant
	}
	if err != nil {
		return "", errors.New("could not resolve development tenant")
	}
	return tenantID, nil
}

func (s *Service) trackUsage(ctx context.Context, tenantID, postID string, usage gemini.Usage) error {
	return insertUsage(ctx, s.pool, tenantID, postID, usage)
}

func (s *Service) recordUnattachedUsage(ctx context.Context, tenantID string, usages ...gemini.Usage) error {
	for _, usage := range usages {
		if err := s.trackUsage(ctx, tenantID, "", usage); err != nil {
			return err
		}
	}
	return nil
}

type usageExecutor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func insertUsage(ctx context.Context, exec usageExecutor, tenantID, postID string, usage gemini.Usage) error {
	if usage.Provider == "" || usage.Model == "" || usage.Operation == "" {
		return errors.New("provider usage metadata is incomplete")
	}
	_, err := exec.Exec(ctx, `
		INSERT INTO ai_usage (tenant_id, post_id, provider, model, operation, input_tokens, output_tokens, latency_ms, success, error_code, cost_usd, cost_basis)
		VALUES ($1::uuid, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8, $9, NULLIF($10,''), $11, $12)`, tenantID, postID,
		usage.Provider, usage.Model, usage.Operation, usage.InputTokens, usage.OutputTokens, usage.Latency.Milliseconds(), usage.Success, usage.ErrorCode, usage.CostUSD, usage.CostBasis)
	return err
}

func formatVector(values []float32) (string, error) {
	parts := make([]string, len(values))
	for i, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return "", errors.New("embedding contains non-finite values")
		}
		parts[i] = strconv.FormatFloat(float64(value), 'f', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}

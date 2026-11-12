package worker

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/providers/gemini"
)

func (w *Worker) process(ctx context.Context, job Job) string {
	if filepath.Base(job.StorageKey) != job.StorageKey || job.StorageKey == "." || job.StorageKey == ".." {
		return "invalid_storage_key"
	}
	imageBytes, err := os.ReadFile(filepath.Join(w.storage, job.StorageKey))
	if err != nil {
		return "image_file_unavailable"
	}
	metadata, analysisUsage, err := w.provider.AnalyzeImage(ctx, imageBytes, job.MIMEType)
	if usageErr := w.persistUsage(ctx, job, analysisUsage); usageErr != nil {
		return "usage_persistence_failed"
	}
	if err != nil {
		return errorCode(analysisUsage, "vision_request_failed")
	}
	if err := metadata.Validate(); err != nil {
		return "invalid_vision_metadata"
	}
	embedding, embeddingUsage, err := w.provider.EmbedImage(ctx, imageBytes, job.MIMEType)
	if usageErr := w.persistUsage(ctx, job, embeddingUsage); usageErr != nil {
		return "usage_persistence_failed"
	}
	if err != nil {
		return errorCode(embeddingUsage, "embedding_request_failed")
	}
	if len(embedding) != gemini.EmbeddingDimensions {
		return "invalid_embedding_dimensions"
	}
	vector, err := formatVector(embedding)
	if err != nil {
		return "invalid_embedding_values"
	}
	attributes, err := json.Marshal(metadata.Attributes)
	if err != nil {
		return "invalid_vision_metadata"
	}
	imageStatus := "processed"
	uncertain := metadata.Confidence < w.floor
	if uncertain {
		imageStatus = "needs_review"
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return "database_persistence_failed"
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `
		INSERT INTO image_analysis (image_id, subject, subject_key, category, attributes, caption, confidence, uncertain, model)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb, $6, $7, $8, $9)
		ON CONFLICT (image_id) DO UPDATE SET subject=EXCLUDED.subject, subject_key=EXCLUDED.subject_key,
		category=EXCLUDED.category, attributes=EXCLUDED.attributes, caption=EXCLUDED.caption,
		confidence=EXCLUDED.confidence, uncertain=EXCLUDED.uncertain, model=EXCLUDED.model, analyzed_at=now()`,
		job.ImageID, metadata.Subject, domain.SubjectKey(metadata.Subject), metadata.Category, string(attributes), metadata.Caption,
		metadata.Confidence, uncertain, analysisUsage.Model)
	if err != nil {
		return "database_persistence_failed"
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO image_embeddings (image_id, tenant_id, embedding, model)
		VALUES ($1::uuid, $2::uuid, $3::vector, $4)
		ON CONFLICT (image_id) DO UPDATE SET embedding=EXCLUDED.embedding, model=EXCLUDED.model, created_at=now()`,
		job.ImageID, job.TenantID, vector, embeddingUsage.Model)
	if err != nil {
		return "database_persistence_failed"
	}
	if _, err := tx.Exec(ctx, `UPDATE images SET status=$2, updated_at=now() WHERE id=$1::uuid`, job.ImageID, imageStatus); err != nil {
		return "database_persistence_failed"
	}
	if _, err := tx.Exec(ctx, `UPDATE jobs SET status='succeeded', locked_at=NULL, last_error=NULL, updated_at=now() WHERE id=$1::uuid`, job.ID); err != nil {
		return "database_persistence_failed"
	}
	if err := tx.Commit(ctx); err != nil {
		return "database_persistence_failed"
	}
	w.logger.InfoContext(ctx, "image processing completed", "image_id", job.ImageID, "status", imageStatus, "confidence", metadata.Confidence)
	return ""
}

func (w *Worker) persistUsage(ctx context.Context, job Job, usage gemini.Usage) error {
	if usage.Provider == "" || usage.Model == "" || usage.Operation == "" {
		return errors.New("provider usage metadata is incomplete")
	}
	writeCtx := ctx
	var cancel context.CancelFunc
	if ctx.Err() != nil {
		writeCtx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}
	_, err := w.pool.Exec(writeCtx, `
		INSERT INTO ai_usage (tenant_id, image_id, provider, model, operation, input_tokens, output_tokens,
			latency_ms, success, error_code, cost_usd, cost_basis)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, ''), $11, $12)`,
		job.TenantID, job.ImageID, usage.Provider, usage.Model, usage.Operation, usage.InputTokens, usage.OutputTokens,
		usage.Latency.Milliseconds(), usage.Success, usage.ErrorCode, usage.CostUSD, usage.CostBasis)
	return err
}

func (w *Worker) retry(ctx context.Context, job Job, code string) error {
	delay := retryDelay(job.Attempts)
	status := "queued"
	if job.Attempts >= job.MaxAttempts {
		status = "failed"
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `UPDATE jobs SET status=$2, available_at=now()+$3*interval '1 second', locked_at=NULL, last_error=$4, updated_at=now() WHERE id=$1::uuid`,
		job.ID, status, delay.Seconds(), code)
	if err != nil {
		return err
	}
	imageStatus := "queued"
	if status == "failed" {
		imageStatus = "failed"
	}
	if _, err := tx.Exec(ctx, `UPDATE images SET status=$2, updated_at=now() WHERE id=$1::uuid`, job.ImageID, imageStatus); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func errorCode(usage gemini.Usage, fallback string) string {
	if usage.ErrorCode != "" && len(usage.ErrorCode) <= 64 && !strings.ContainsAny(usage.ErrorCode, "\r\n") {
		return usage.ErrorCode
	}
	return fallback
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

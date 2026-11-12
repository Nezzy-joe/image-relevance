package worker

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/Nezzy-joe/image-relevance/internal/providers/gemini"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Provider interface {
	AnalyzeImage(context.Context, []byte, string) (domain.ImageMetadata, gemini.Usage, error)
	EmbedText(context.Context, string) ([]float32, gemini.Usage, error)
	EmbedImage(context.Context, []byte, string) ([]float32, gemini.Usage, error)
}

type Job struct {
	ID, TenantID, ImageID, StorageKey, MIMEType string
	Attempts, MaxAttempts                       int
}

type Worker struct {
	pool       *pgxpool.Pool
	provider   Provider
	storage    string
	concurrent int
	poll       time.Duration
	floor      float64
	logger     *slog.Logger
}

func New(pool *pgxpool.Pool, provider Provider, storage string, concurrency int, poll time.Duration, confidenceFloor float64, logger *slog.Logger) *Worker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Worker{pool: pool, provider: provider, storage: storage, concurrent: concurrency, poll: poll, floor: confidenceFloor, logger: logger}
}

func (w *Worker) Run(ctx context.Context) {
	workers := w.concurrent
	if workers < 1 {
		workers = 1
	}
	var group syncGroup
	for i := 0; i < workers; i++ {
		group.add()
		go func() {
			defer group.done()
			w.loop(ctx)
		}()
	}
	group.wait()
}

func (w *Worker) loop(ctx context.Context) {
	for ctx.Err() == nil {
		job, err := w.claim(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.ErrorContext(ctx, "could not claim image job", slog.String("failure", "database_claim_failed"))
			if !wait(ctx, w.poll) {
				return
			}
			continue
		}
		if job == nil {
			if !wait(ctx, w.poll) {
				return
			}
			continue
		}
		if code := w.process(ctx, *job); code != "" {
			if ctx.Err() != nil {
				w.release(*job)
				return
			}
			if err := w.retry(ctx, *job, code); err != nil && ctx.Err() == nil {
				w.logger.ErrorContext(ctx, "could not update failed image job", slog.String("failure", "job_retry_update_failed"))
			}
			w.logger.WarnContext(ctx, "image processing attempt failed", slog.String("job_id", job.ID), slog.String("failure", code), slog.Int("attempt", job.Attempts))
		}
	}
}

func (w *Worker) release(job Job) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE jobs SET status='queued', attempts=GREATEST(attempts-1,0), available_at=now(), locked_at=NULL, updated_at=now() WHERE id=$1::uuid AND status='running'`, job.ID); err != nil {
		return
	}
	if _, err := tx.Exec(ctx, `UPDATE images SET status='queued', updated_at=now() WHERE id=$1::uuid`, job.ImageID); err != nil {
		return
	}
	_ = tx.Commit(ctx)
}

func (w *Worker) claim(ctx context.Context) (*Job, error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// A worker that disappears is recoverable after five minutes. Exhausted stale
	// jobs are marked failed before the next available job is claimed.
	rows, err := tx.Query(ctx, `
		UPDATE jobs SET status='failed', locked_at=NULL, updated_at=now(), last_error='worker_lost'
		WHERE status='running' AND locked_at < now() - interval '5 minutes' AND attempts >= max_attempts
		RETURNING image_id::text`)
	if err != nil {
		return nil, err
	}
	var exhausted []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		exhausted = append(exhausted, id)
	}
	rows.Close()
	for _, id := range exhausted {
		if _, err := tx.Exec(ctx, `UPDATE images SET status='failed', updated_at=now() WHERE id=$1::uuid`, id); err != nil {
			return nil, err
		}
	}
	var job Job
	err = tx.QueryRow(ctx, `
		WITH candidate AS (
			SELECT j.id FROM jobs j
			WHERE j.attempts < j.max_attempts AND (
				(j.status='queued' AND j.available_at <= now()) OR
				(j.status='running' AND j.locked_at < now() - interval '5 minutes')
			)
			ORDER BY j.available_at, j.created_at
			FOR UPDATE SKIP LOCKED LIMIT 1
		)
		UPDATE jobs j SET status='running', attempts=j.attempts+1, locked_at=now(), updated_at=now()
		FROM candidate c, images i
		WHERE j.id=c.id AND i.id=j.image_id
		RETURNING j.id::text, j.tenant_id::text, j.image_id::text, i.storage_key, i.mime_type, j.attempts, j.max_attempts`).
		Scan(&job.ID, &job.TenantID, &job.ImageID, &job.StorageKey, &job.MIMEType, &job.Attempts, &job.MaxAttempts)
	if err == pgx.ErrNoRows {
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE images SET status='processing', updated_at=now() WHERE id=$1::uuid`, job.ImageID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &job, nil
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	seconds := math.Pow(2, float64(attempt-1))
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

// syncGroup is the small subset of sync.WaitGroup used to keep worker state local.
type syncGroup struct{ wg sync.WaitGroup }

func (g *syncGroup) add()  { g.wg.Add(1) }
func (g *syncGroup) done() { g.wg.Done() }
func (g *syncGroup) wait() { g.wg.Wait() }

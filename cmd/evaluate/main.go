package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nezzy-joe/image-relevance/internal/config"
	"github.com/Nezzy-joe/image-relevance/internal/database"
	"github.com/Nezzy-joe/image-relevance/internal/evaluation"
	"github.com/Nezzy-joe/image-relevance/internal/matching"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	imagesPath := flag.String("images", "", "CSV image manifest (image_id,category,source_url,license)")
	articlesPath := flag.String("articles", "", "CSV labels (post_id,expected_image_ids,negative_image_ids,scenario)")
	flag.Parse()
	if *imagesPath == "" || *articlesPath == "" {
		fmt.Fprintln(os.Stderr, "usage: evaluate -images images.csv -articles articles.csv")
		os.Exit(2)
	}
	if err := run(*imagesPath, *articlesPath); err != nil {
		log.Printf("evaluation failed: %v", err)
		os.Exit(1)
	}
}

func run(imagesPath, articlesPath string) error {
	manifest, err := evaluation.LoadManifest(imagesPath, articlesPath)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	connectCtx, stop := context.WithTimeout(ctx, cfg.DatabaseConnect)
	pool, err := database.OpenPool(connectCtx, database.PoolConfig{URL: cfg.DatabaseURL, MaxConns: cfg.DatabaseMaxConns})
	stop()
	if err != nil {
		return errors.New("database connection failed; verify the local PostgreSQL service and DATABASE_URL")
	}
	defer pool.Close()
	if err := validateImageSet(ctx, pool, manifest); err != nil {
		return err
	}
	service := matching.New(pool, nil, cfg.ConfidenceFloor, cfg.SimilarityFloor)
	report, err := evaluation.Evaluate(ctx, manifest, service.MatchCandidates)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func validateImageSet(ctx context.Context, pool *pgxpool.Pool, manifest evaluation.Manifest) error {
	var tenantID string
	err := pool.QueryRow(ctx, `SELECT id::text FROM tenants WHERE name=$1 AND status='active' ORDER BY created_at LIMIT 1`, "image-relevance development tenant").Scan(&tenantID)
	if err != nil {
		return errors.New("development tenant is not seeded")
	}
	ids := make([]string, 0, len(manifest.Images))
	for _, image := range manifest.Images {
		ids = append(ids, image.ID)
	}
	var imageCount int
	err = pool.QueryRow(ctx, `
		SELECT count(DISTINCT i.id)
		FROM images i JOIN image_analysis a ON a.image_id=i.id
		WHERE i.tenant_id=$1::uuid AND i.id=ANY($2::uuid[]) AND i.status='processed' AND NOT a.uncertain`, tenantID, ids).
		Scan(&imageCount)
	if err != nil {
		return errors.New("could not verify the processed evaluation image set")
	}
	categories := make(map[string]struct{}, len(manifest.Images))
	for _, image := range manifest.Images {
		categories[image.Category] = struct{}{}
	}
	if imageCount < 40 || len(categories) < 4 {
		return fmt.Errorf("evaluation database needs at least 40 non-uncertain processed manifest images across four verified manifest categories; found %d images across %d categories", imageCount, len(categories))
	}
	return nil
}

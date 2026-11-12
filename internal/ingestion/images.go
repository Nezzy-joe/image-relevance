package ingestion

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Nezzy-joe/image-relevance/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrImageTooLarge = domain.ErrImageTooLarge
	ErrInvalidImage  = domain.ErrInvalidImage
	ErrNoTenant      = domain.ErrNoTenant
	ErrImageNotFound = domain.ErrImageNotFound
)

const (
	developmentTenantName = "image-relevance development tenant"
	maxImagePixels        = int64(40_000_000)
)

type ImageService struct {
	pool     *pgxpool.Pool
	root     string
	maxBytes int64
}

func NewImageService(pool *pgxpool.Pool, root string, maxBytes int64) *ImageService {
	return &ImageService{pool: pool, root: root, maxBytes: maxBytes}
}

func (s *ImageService) Upload(ctx context.Context, data []byte, declaredMIME string) (domain.ImageUploadResult, error) {
	if int64(len(data)) > s.maxBytes {
		return domain.ImageUploadResult{}, ErrImageTooLarge
	}
	mimeType, extension, err := validateImage(data, declaredMIME)
	if err != nil {
		return domain.ImageUploadResult{}, err
	}
	checksum := sha256.Sum256(data)
	sha := hex.EncodeToString(checksum[:])
	storageKey := sha + extension
	if err := s.store(storageKey, data); err != nil {
		return domain.ImageUploadResult{}, errors.New("could not persist uploaded image")
	}

	tenantID, err := s.developmentTenantID(ctx)
	if err != nil {
		return domain.ImageUploadResult{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.ImageUploadResult{}, errors.New("could not begin image persistence transaction")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var imageID string
	var status string
	err = tx.QueryRow(ctx, `
		INSERT INTO images (tenant_id, sha256, storage_key, mime_type, size_bytes, status)
		VALUES ($1::uuid, $2, $3, $4, $5, 'queued')
		ON CONFLICT (tenant_id, sha256) DO NOTHING
		RETURNING id::text, status`, tenantID, sha, storageKey, mimeType, len(data)).Scan(&imageID, &status)
	if err == pgx.ErrNoRows {
		if err := tx.QueryRow(ctx, `SELECT id::text, status FROM images WHERE tenant_id=$1::uuid AND sha256=$2 FOR UPDATE`, tenantID, sha).Scan(&imageID, &status); err != nil {
			return domain.ImageUploadResult{}, errors.New("could not retrieve existing image")
		}
		if status == "failed" {
			if _, err := tx.Exec(ctx, `UPDATE images SET status='queued', updated_at=now() WHERE id=$1::uuid`, imageID); err != nil {
				return domain.ImageUploadResult{}, errors.New("could not requeue failed image")
			}
			status = "queued"
		} else {
			if err := tx.Commit(ctx); err != nil {
				return domain.ImageUploadResult{}, errors.New("could not complete image lookup")
			}
			return domain.ImageUploadResult{ImageID: imageID, Status: status, Existing: true}, nil
		}
	} else if err != nil {
		return domain.ImageUploadResult{}, errors.New("could not save image record")
	}

	var jobID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO jobs (tenant_id, image_id, job_type)
		VALUES ($1::uuid, $2::uuid, 'analyze_image')
		RETURNING id::text`, tenantID, imageID).Scan(&jobID); err != nil {
		return domain.ImageUploadResult{}, errors.New("could not queue image processing")
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.ImageUploadResult{}, errors.New("could not commit image upload")
	}
	return domain.ImageUploadResult{ImageID: imageID, JobID: jobID, Status: status, Existing: false}, nil
}

func (s *ImageService) Get(ctx context.Context, imageID string) (domain.ImageRecord, error) {
	tenantID, err := s.developmentTenantID(ctx)
	if err != nil {
		return domain.ImageRecord{}, err
	}
	var record domain.ImageRecord
	var attrs []byte
	var subject, category, caption *string
	var confidence *float64
	err = s.pool.QueryRow(ctx, `
		SELECT i.id::text, i.status, i.sha256, i.mime_type, i.size_bytes, i.created_at,
		       a.subject, a.category, a.attributes, a.caption, a.confidence
		FROM images i
	LEFT JOIN image_analysis a ON a.image_id=i.id
		WHERE i.id=$1::uuid AND i.tenant_id=$2::uuid`, imageID, tenantID).
		Scan(&record.ID, &record.Status, &record.SHA256, &record.MIMEType, &record.SizeBytes, &record.CreatedAt,
			&subject, &category, &attrs, &caption, &confidence)
	if err == pgx.ErrNoRows {
		return domain.ImageRecord{}, ErrImageNotFound
	}
	if err != nil {
		return domain.ImageRecord{}, errors.New("could not retrieve image status")
	}
	if subject != nil && category != nil && caption != nil && confidence != nil {
		metadata := &domain.ImageMetadata{Subject: *subject, Category: *category, Caption: *caption, Confidence: *confidence}
		if err := json.Unmarshal(attrs, &metadata.Attributes); err != nil {
			return domain.ImageRecord{}, errors.New("stored image metadata is invalid")
		}
		record.Metadata = metadata
	}
	return record, nil
}

func (s *ImageService) developmentTenantID(ctx context.Context) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id::text FROM tenants WHERE name=$1 AND status='active' ORDER BY created_at LIMIT 1`, developmentTenantName).Scan(&id)
	if err == pgx.ErrNoRows {
		return "", ErrNoTenant
	}
	if err != nil {
		return "", errors.New("could not resolve development tenant")
	}
	return id, nil
}

func (s *ImageService) store(key string, data []byte) error {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(s.root, key))
}

func validateImage(data []byte, declaredMIME string) (string, string, error) {
	if len(data) == 0 {
		return "", "", ErrInvalidImage
	}
	header := data
	if len(header) > 512 {
		header = header[:512]
	}
	detected := http.DetectContentType(header)
	_, format, err := decodeImageConfig(data)
	if err != nil {
		return "", "", err
	}
	var extension string
	switch format {
	case "jpeg":
		extension = ".jpg"
	case "png":
		extension = ".png"
	default:
		return "", "", ErrInvalidImage
	}
	if (format == "jpeg" && detected != "image/jpeg") || (format == "png" && detected != "image/png") {
		return "", "", ErrInvalidImage
	}
	if declaredMIME != "" && declaredMIME != "application/octet-stream" && declaredMIME != detected {
		return "", "", ErrInvalidImage
	}
	return detected, extension, nil
}

func decodeImageConfig(data []byte) (image.Config, string, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || config.Width < 1 || config.Height < 1 {
		return image.Config{}, "", ErrInvalidImage
	}
	if int64(config.Width)*int64(config.Height) > maxImagePixels {
		return image.Config{}, "", ErrInvalidImage
	}
	return config, format, nil
}

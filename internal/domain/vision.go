package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"
)

type ImageMetadata struct {
	Subject    string   `json:"subject"`
	Category   string   `json:"category"`
	Attributes []string `json:"attributes"`
	Caption    string   `json:"caption"`
	Confidence float64  `json:"confidence"`
}

type SubjectResolution struct {
	Subject    string  `json:"subject"`
	Confidence float64 `json:"confidence"`
}

type ImageRecord struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"`
	SHA256    string         `json:"sha256"`
	MIMEType  string         `json:"mime_type"`
	SizeBytes int64          `json:"size_bytes"`
	Metadata  *ImageMetadata `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type ImageUploadResult struct {
	ImageID  string `json:"image_id"`
	JobID    string `json:"job_id,omitempty"`
	Status   string `json:"status"`
	Existing bool   `json:"existing"`
}

func ParseImageMetadata(data []byte) (ImageMetadata, error) {
	if len(data) == 0 || len(data) > 16*1024 {
		return ImageMetadata{}, errors.New("metadata JSON has an invalid size")
	}
	var wire struct {
		Subject    string   `json:"subject"`
		Category   string   `json:"category"`
		Attributes []string `json:"attributes"`
		Caption    string   `json:"caption"`
		Confidence *float64 `json:"confidence"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return ImageMetadata{}, errors.New("metadata is not valid structured JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return ImageMetadata{}, errors.New("metadata must contain exactly one JSON object")
	}
	if wire.Confidence == nil || wire.Attributes == nil {
		return ImageMetadata{}, errors.New("metadata is missing required fields")
	}
	metadata := ImageMetadata{
		Subject: strings.TrimSpace(wire.Subject), Category: strings.TrimSpace(wire.Category),
		Attributes: wire.Attributes, Caption: strings.TrimSpace(wire.Caption), Confidence: *wire.Confidence,
	}
	if err := metadata.Validate(); err != nil {
		return ImageMetadata{}, err
	}
	return metadata, nil
}

func (m ImageMetadata) Validate() error {
	if m.Subject == "" || len(m.Subject) > 120 {
		return errors.New("subject must contain 1 to 120 characters")
	}
	if m.Category == "" || len(m.Category) > 64 {
		return errors.New("category must contain 1 to 64 characters")
	}
	if m.Caption == "" || len(m.Caption) > 500 {
		return errors.New("caption must contain 1 to 500 characters")
	}
	if len(m.Attributes) > 20 {
		return errors.New("attributes may contain at most 20 values")
	}
	for _, attribute := range m.Attributes {
		if strings.TrimSpace(attribute) == "" || len(attribute) > 80 {
			return errors.New("each attribute must contain 1 to 80 characters")
		}
	}
	if math.IsNaN(m.Confidence) || math.IsInf(m.Confidence, 0) || m.Confidence < 0 || m.Confidence > 1 {
		return errors.New("confidence must be a finite number between 0 and 1")
	}
	return nil
}

func ParseSubjectResolution(data []byte) (SubjectResolution, error) {
	if len(data) == 0 || len(data) > 4*1024 {
		return SubjectResolution{}, errors.New("subject JSON has an invalid size")
	}
	var wire struct {
		Subject    string   `json:"subject"`
		Confidence *float64 `json:"confidence"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return SubjectResolution{}, errors.New("subject is not valid structured JSON")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return SubjectResolution{}, errors.New("subject must contain exactly one JSON object")
	}
	if wire.Confidence == nil {
		return SubjectResolution{}, errors.New("subject confidence is missing")
	}
	result := SubjectResolution{Subject: strings.TrimSpace(wire.Subject), Confidence: *wire.Confidence}
	if result.Subject == "" || len(result.Subject) > 120 {
		return SubjectResolution{}, errors.New("subject must contain 1 to 120 characters")
	}
	if math.IsNaN(result.Confidence) || math.IsInf(result.Confidence, 0) || result.Confidence < 0 || result.Confidence > 1 {
		return SubjectResolution{}, errors.New("confidence must be a finite number between 0 and 1")
	}
	return result, nil
}

func SubjectKey(subject string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(strings.TrimSpace(subject)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		} else {
			space = true
		}
	}
	return b.String()
}

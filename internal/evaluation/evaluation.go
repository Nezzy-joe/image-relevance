package evaluation

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/Nezzy-joe/image-relevance/internal/matching"
)

type Image struct {
	ID, Category, SourceURL, License string
}

type Article struct {
	PostID, Scenario   string
	Expected, Negative []string
}

type Manifest struct {
	Images   []Image
	Articles []Article
}

type Report struct {
	ImageCount                 int     `json:"image_count"`
	CategoryCount              int     `json:"category_count"`
	ArticleCount               int     `json:"article_count"`
	Top1Precision              float64 `json:"top1_precision"`
	RecommendationCoverage     float64 `json:"recommendation_coverage"`
	AcceptedRecommendationPrec float64 `json:"accepted_recommendation_precision"`
	IncorrectAcceptedMatches   int     `json:"incorrect_accepted_matches"`
	Abstentions                int     `json:"abstentions"`
}

func LoadManifest(imagesPath, articlesPath string) (Manifest, error) {
	images, err := loadImages(imagesPath)
	if err != nil {
		return Manifest{}, err
	}
	articles, err := loadArticles(articlesPath)
	if err != nil {
		return Manifest{}, err
	}
	if len(images) < 40 {
		return Manifest{}, errors.New("image manifest must contain at least 40 unique licensed images")
	}
	categories := map[string]bool{}
	imageIDs := map[string]bool{}
	for _, image := range images {
		imageIDs[image.ID] = true
		categories[image.Category] = true
	}
	if len(categories) < 4 {
		return Manifest{}, errors.New("image manifest must cover at least four categories")
	}
	if len(articles) < 10 {
		return Manifest{}, errors.New("article labels must contain at least 10 unique articles")
	}
	articleIDs, scenarios := map[string]bool{}, map[string]int{}
	for _, article := range articles {
		if articleIDs[article.PostID] {
			return Manifest{}, fmt.Errorf("duplicate article post_id %q", article.PostID)
		}
		articleIDs[article.PostID] = true
		scenarios[article.Scenario]++
		for _, id := range append(append([]string{}, article.Expected...), article.Negative...) {
			if !imageIDs[id] {
				return Manifest{}, fmt.Errorf("article %q references image %q missing from the image manifest", article.PostID, id)
			}
		}
		if article.Scenario == "no_suitable" && len(article.Expected) > 0 {
			return Manifest{}, fmt.Errorf("no_suitable article %q cannot have expected images", article.PostID)
		}
		if article.Scenario != "no_suitable" && len(article.Expected) == 0 {
			return Manifest{}, fmt.Errorf("article %q needs at least one expected image", article.PostID)
		}
		negatives := makeSet(article.Negative)
		for _, id := range article.Expected {
			if negatives[id] {
				return Manifest{}, fmt.Errorf("article %q marks image %q both expected and negative", article.PostID, id)
			}
		}
	}
	if scenarios["positive"] == 0 || scenarios["mismatch_guard"] == 0 || scenarios["no_suitable"] == 0 {
		return Manifest{}, errors.New("article labels need positive, mismatch_guard, and no_suitable scenarios")
	}
	for _, article := range articles {
		if article.Scenario == "mismatch_guard" && len(article.Negative) == 0 {
			return Manifest{}, fmt.Errorf("mismatch_guard article %q must label negative images", article.PostID)
		}
	}
	return Manifest{Images: images, Articles: articles}, nil
}

func Evaluate(ctx context.Context, manifest Manifest, match func(context.Context, string, []string) (matching.MatchResult, error)) (Report, error) {
	imageIDs := make([]string, 0, len(manifest.Images))
	for _, image := range manifest.Images {
		imageIDs = append(imageIDs, image.ID)
	}
	categorySet := makeSet(func() []string {
		values := make([]string, 0, len(manifest.Images))
		for _, image := range manifest.Images {
			values = append(values, image.Category)
		}
		return values
	}())
	report := Report{ImageCount: len(manifest.Images), CategoryCount: len(categorySet), ArticleCount: len(manifest.Articles)}
	var top1Correct, top1Cases, recommendations, accepted, correctAccepted int
	for _, article := range manifest.Articles {
		result, err := match(ctx, article.PostID, imageIDs)
		if err != nil {
			return Report{}, fmt.Errorf("matching article %q failed: %w", article.PostID, err)
		}
		if article.Scenario != "no_suitable" {
			top1Cases++
			if len(result.Suggestions) > 0 && contains(article.Expected, result.Suggestions[0].ImageID) {
				top1Correct++
			}
		}
		if len(result.Suggestions) == 0 {
			report.Abstentions++
		} else {
			recommendations++
		}
		for _, candidate := range result.Suggestions {
			accepted++
			if contains(article.Expected, candidate.ImageID) {
				correctAccepted++
			} else {
				report.IncorrectAcceptedMatches++
			}
		}
	}
	if top1Cases > 0 {
		report.Top1Precision = float64(top1Correct) / float64(top1Cases)
	}
	if len(manifest.Articles) > 0 {
		report.RecommendationCoverage = float64(recommendations) / float64(len(manifest.Articles))
	}
	if accepted > 0 {
		report.AcceptedRecommendationPrec = float64(correctAccepted) / float64(accepted)
	}
	return report, nil
}

func loadImages(path string) ([]Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image manifest: %w", err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 4
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read image manifest header: %w", err)
	}
	if strings.Join(header, ",") != "image_id,category,source_url,license" {
		return nil, errors.New("image manifest header must be image_id,category,source_url,license")
	}
	seen := map[string]bool{}
	var images []Image
	for row := 2; ; row++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read image manifest row %d: %w", row, err)
		}
		image := Image{ID: strings.TrimSpace(record[0]), Category: strings.TrimSpace(record[1]), SourceURL: strings.TrimSpace(record[2]), License: strings.TrimSpace(record[3])}
		parsedURL, parseErr := url.ParseRequestURI(image.SourceURL)
		if image.ID == "" || image.Category == "" || image.License == "" || parseErr != nil || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") || parsedURL.Host == "" {
			return nil, fmt.Errorf("image manifest row %d needs an ID, category, HTTP(S) source URL, and license", row)
		}
		if seen[image.ID] {
			return nil, fmt.Errorf("duplicate image_id %q", image.ID)
		}
		seen[image.ID] = true
		images = append(images, image)
	}
	return images, nil
}

func loadArticles(path string) ([]Article, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open article labels: %w", err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 4
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read article labels header: %w", err)
	}
	if strings.Join(header, ",") != "post_id,expected_image_ids,negative_image_ids,scenario" {
		return nil, errors.New("article labels header must be post_id,expected_image_ids,negative_image_ids,scenario")
	}
	var articles []Article
	for row := 2; ; row++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read article labels row %d: %w", row, err)
		}
		article := Article{PostID: strings.TrimSpace(record[0]), Expected: splitIDs(record[1]), Negative: splitIDs(record[2]), Scenario: strings.TrimSpace(record[3])}
		if article.PostID == "" || (article.Scenario != "positive" && article.Scenario != "mismatch_guard" && article.Scenario != "no_suitable") {
			return nil, fmt.Errorf("article labels row %d has an invalid post_id or scenario", row)
		}
		articles = append(articles, article)
	}
	return articles, nil
}

func splitIDs(value string) []string {
	var out []string
	for _, item := range strings.Split(value, "|") {
		if id := strings.TrimSpace(item); id != "" {
			out = append(out, id)
		}
	}
	return out
}

func makeSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

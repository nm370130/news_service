package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/nitesh/news_service/internal/llm"
	"github.com/nitesh/news_service/pkg/models"
	"github.com/redis/go-redis/v9"
)

type ArticleStore interface {
	SaveMany([]*models.Article) error
	Search(q string, limit int) ([]*models.Article, error)
	FindByCategory(category string, limit int) ([]*models.Article, error)
	All(limit int) ([]*models.Article, error)
	GetByIDs([]string) ([]*models.Article, error)

	UpdateLLMSummary(id string, summary string) error
	Nearby(lat, lon, radiusKm float64, limit int) ([]*models.Article, error)
}

type Service struct {
	repo      ArticleStore
	rdb       *redis.Client
	llmClient *llm.Client
}

// func NewService(repo ArticleStore, rdb *redis.Client) *Service {
//     return &Service{repo: repo, rdb: rdb}
// }

func NewService(repo ArticleStore, rdb *redis.Client, llmClient *llm.Client) *Service {
	return &Service{repo: repo, rdb: rdb, llmClient: llmClient}
}

// SummarizeArticle generates a short summary for an article (2-4 sentences),
// saves it into the DB and returns the summary.
func (s *Service) SummarizeArticle(ctx context.Context, id string) (string, error) {
	// fetch article
	arts, err := s.repo.GetByIDs([]string{id})
	if err != nil {
		return "", fmt.Errorf("fetch article: %w", err)
	}
	if len(arts) == 0 {
		return "", fmt.Errorf("article not found")
	}
	art := arts[0]

	// pick best text to summarize (use Description if present, otherwise Title)
	content := art.Description
	if content == "" {
		content = art.Title
	}
	// optional: truncate content if extremely large to keep token cost reasonable
	// but the LLM client will still accept it. We'll limit to first 30k chars
	if len(content) > 30000 {
		content = content[:30000]
	}

	// call the llm client
	summary, err := s.llmClient.SummarizeArticleText(ctx, art.Title, content)
	if err != nil {
		return "", fmt.Errorf("llm summarize: %w", err)
	}
	art.LLMSummary = summary
	if err := s.repo.SaveMany([]*models.Article{art}); err != nil {
		return "", fmt.Errorf("save summary: %w", err)
	}

	// persist summary
	if err := s.repo.UpdateLLMSummary(art.ID, summary); err != nil {
		return "", fmt.Errorf("save summary: %w", err)
	}

	return summary, nil
}

// Ingest articles
func (s *Service) Ingest(ctx context.Context, articles []*models.Article) error {
	// set defaults
	for _, a := range articles {
		if a.PublishedAt.IsZero() {
			a.PublishedAt = time.Now()
		}
	}
	return s.repo.SaveMany(articles)
}

func (s *Service) Search(ctx context.Context, q string, limit int) ([]*models.Article, error) {
	return s.repo.Search(q, limit)
}

func (s *Service) Category(ctx context.Context, category string, limit int) ([]*models.Article, error) {
	return s.repo.FindByCategory(category, limit)
}

func (s *Service) Trending(ctx context.Context, limit int) ([]*models.Article, error) {
	return s.repo.All(limit)
}

// func (s *Service) Nearby(ctx context.Context, lat, lon, radiusKm float64, limit int) ([]*models.Article, error) {
// 	all, err := s.repo.All(1000) // fetch candidates (for small dataset)
// 	if err != nil {
// 		return nil, err
// 	}
// 	out := []*models.Article{}
// 	for _, a := range all {
// 		if a.Latitude == 0 && a.Longitude == 0 {
// 			continue
// 		}
// 		dist := haversineKm(lat, lon, a.Latitude, a.Longitude)
// 		if dist <= radiusKm {
// 			a.DistanceKm = dist
// 			out = append(out, a)
// 		}
// 		if limit > 0 && len(out) >= limit {
// 			break
// 		}
// 	}
// 	// sort by distance
// 	sort.Slice(out, func(i, j int) bool { return out[i].DistanceKm < out[j].DistanceKm })
// 	return out, nil
// }

func (s *Service) Nearby(ctx context.Context, lat, lon, radiusKm float64, limit int) ([]*models.Article, error) {
	// call DB-side optimized query
	return s.repo.Nearby(lat, lon, radiusKm, limit)
}

// helpers
func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLon := toRad(lon2 - lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

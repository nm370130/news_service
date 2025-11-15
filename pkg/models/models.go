package models

import (
	"time"

	dbtypes "github.com/nitesh/news_service/internal/db"
)

// Article represents a news article record used across the service.
type Article struct {
	ID          string           `db:"id" json:"id"`
	Title       string           `db:"title" json:"title"`
	Description string           `db:"description" json:"description"`
	URL         string           `db:"url" json:"url"`
	PublishedAt time.Time        `db:"published_at" json:"published_at"`
	Source      string           `db:"source" json:"source"`
	Categories  dbtypes.StringSlice `db:"categories" json:"categories"`
	Relevance   float64          `db:"relevance_score" json:"relevance_score"`
	Latitude    float64          `db:"latitude" json:"latitude"`
	Longitude   float64          `db:"longitude" json:"longitude"`
	LLMSummary  string           `db:"llm_summary" json:"llm_summary"`

	// DistanceKm is set at runtime by the Nearby function (not persisted).
	DistanceKm  float64          `db:"distance_km" json:"distance_km,omitempty"`
}
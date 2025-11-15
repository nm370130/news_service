package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	dbtypes "github.com/nitesh/news_service/internal/db"
	"github.com/nitesh/news_service/pkg/models"
)

type PgStore struct {
	db *sqlx.DB
}

func NewPgStore(db *sql.DB) *PgStore {
	return &PgStore{db: sqlx.NewDb(db, "postgres")}
}

func RunMigrations(db *sql.DB) error {
	initSQL := `
CREATE TABLE IF NOT EXISTS articles(
  id UUID PRIMARY KEY,
  title TEXT,
  description TEXT,
  url TEXT,
  published_at TIMESTAMP,
  source TEXT,
  categories JSONB,
  relevance_score DOUBLE PRECISION DEFAULT 0,
  latitude DOUBLE PRECISION,
  longitude DOUBLE PRECISION,
  llm_summary TEXT
);

CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at);
CREATE INDEX IF NOT EXISTS idx_articles_relevance ON articles(relevance_score);
CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source);
-- GIN index for jsonb array search on categories
CREATE INDEX IF NOT EXISTS idx_articles_categories ON articles USING GIN (categories);
`
	_, err := db.Exec(initSQL)
	return err
}

// SaveMany replaces any NamedExec-based insert for articles and writes categories as jsonb.
// It expects models.Article.Categories to be of type dbtypes.StringSlice (implements driver.Valuer).
func (p *PgStore) SaveMany(articles []*models.Article) error {
	tx, err := p.db.Beginx()
	if err != nil {
		return err
	}

	stmt := `
INSERT INTO articles (id, title, description, url, published_at, source, categories, relevance_score, latitude, longitude, llm_summary)
VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11)
ON CONFLICT (id) DO UPDATE SET
 title=EXCLUDED.title,
 description=EXCLUDED.description,
 url=EXCLUDED.url,
 published_at=EXCLUDED.published_at,
 source=EXCLUDED.source,
 categories=EXCLUDED.categories,
 relevance_score=EXCLUDED.relevance_score,
 latitude=EXCLUDED.latitude,
 longitude=EXCLUDED.longitude,
 llm_summary=EXCLUDED.llm_summary;
`

	for _, a := range articles {
		if a.ID == "" {
			a.ID = uuid.New().String()
		}
		// Ensure categories is non-nil; dbtypes.StringSlice marshals nil -> []
		if a.Categories == nil {
			a.Categories = dbtypes.StringSlice{}
		}
		if a.PublishedAt.IsZero() {
			a.PublishedAt = time.Now().UTC()
		}

		_, err := tx.Exec(stmt,
			a.ID,
			a.Title,
			a.Description,
			a.URL,
			a.PublishedAt,
			a.Source,
			a.Categories, // dbtypes.StringSlice -> Value() -> JSON string
			a.Relevance,
			a.Latitude,
			a.Longitude,
			a.LLMSummary,
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("insert article id=%s: %w", a.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (p *PgStore) Search(q string, limit int) ([]*models.Article, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	like := "%%%s%%"
	like = fmt.Sprintf(like, q)
	rows := []*models.Article{}
	query := `
SELECT id,title,description,url,published_at,source,categories,relevance_score,latitude,longitude,llm_summary
FROM articles
WHERE title ILIKE $1 OR description ILIKE $1
ORDER BY relevance_score DESC, published_at DESC
LIMIT $2
`
	err := p.db.Select(&rows, query, like, limit)
	return rows, err
}

func (p *PgStore) FindByCategory(category string, limit int) ([]*models.Article, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	rows := []*models.Article{}
	// For jsonb array of strings, use @> operator to check containment.
	// We build a json array with a single element '["category"]' and check categories @> that array.
	query := `
SELECT id,title,description,url,published_at,source,categories,relevance_score,latitude,longitude,llm_summary
FROM articles
WHERE categories @> ('["' || $1 || '"]')::jsonb
ORDER BY relevance_score DESC, published_at DESC
LIMIT $2
`
	err := p.db.Select(&rows, query, category, limit)
	return rows, err
}

func (p *PgStore) All(limit int) ([]*models.Article, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows := []*models.Article{}
	query := `
SELECT id,title,description,url,published_at,source,categories,relevance_score,latitude,longitude,llm_summary
FROM articles
ORDER BY relevance_score DESC, published_at DESC
LIMIT $1
`
	err := p.db.Select(&rows, query, limit)
	return rows, err
}

func (p *PgStore) GetByIDs(ids []string) ([]*models.Article, error) {
	if len(ids) == 0 {
		return []*models.Article{}, nil
	}

	rows := []*models.Article{}

	// If only one id was requested, use a simple scalar parameter (avoids array conversion)
	if len(ids) == 1 {
		query := `
SELECT id,title,description,url,published_at,source,categories,relevance_score,latitude,longitude,llm_summary
FROM articles
WHERE id = $1
LIMIT 1
`
		err := p.db.Select(&rows, query, ids[0])
		return rows, err
	}

	// For multiple ids, pass a Postgres array. Cast to uuid[] for UUID columns.
	query := `
SELECT id,title,description,url,published_at,source,categories,relevance_score,latitude,longitude,llm_summary
FROM articles
WHERE id = ANY($1::uuid[])
`
	// IMPORTANT: use github.com/lib/pq and pass pq.Array(ids)
	err := p.db.Select(&rows, query, pqArray(ids))
	return rows, err
}

// pqArray helper: sqlx.Select handles pq.Array when using database/sql driver
// but to avoid adding pq import here we marshal a simple interface that sqlx accepts.
// We define pqArray as an alias for compatibility; if needed switch to pq.Array(ids).
func pqArray(a []string) interface{} {
	return a
}

func (p *PgStore) UpdateLLMSummary(id string, summary string) error {
	// use ExecContext if you prefer ctx-aware; keep simple for now
	_, err := p.db.Exec("UPDATE articles SET llm_summary = $1 WHERE id = $2", summary, id)
	return err
}

func (p *PgStore) Nearby(lat, lon, radiusKm float64, limit int) ([]*models.Article, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	// Haversine formula computed in subquery to avoid repeating calculation
	query := `
SELECT id, title, description, url, published_at, source, categories, relevance_score, latitude, longitude, llm_summary, distance_km
FROM (
  SELECT
    id, title, description, url, published_at, source, categories, relevance_score, latitude, longitude, llm_summary,
    (6371 * acos(
        cos(radians($1)) * cos(radians(latitude)) * cos(radians(longitude) - radians($2)) +
        sin(radians($1)) * sin(radians(latitude))
    )) AS distance_km
  FROM articles
  WHERE latitude IS NOT NULL AND longitude IS NOT NULL
) AS t
WHERE distance_km <= $3
ORDER BY distance_km ASC
LIMIT $4;
`

	rows := []*models.Article{}
	err := p.db.Select(&rows, query, lat, lon, radiusKm, limit)
	return rows, err
}

CREATE TABLE IF NOT EXISTS articles(
  id UUID PRIMARY KEY,
  title TEXT,
  description TEXT,
  url TEXT,
  published_at TIMESTAMP,
  source TEXT,
  categories TEXT[],
  relevance_score DOUBLE PRECISION DEFAULT 0,
  latitude DOUBLE PRECISION,
  longitude DOUBLE PRECISION,
  llm_summary TEXT
);

CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at);
CREATE INDEX IF NOT EXISTS idx_articles_relevance ON articles(relevance_score);
CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source);
CREATE INDEX IF NOT EXISTS idx_articles_categories ON articles USING GIN (categories);

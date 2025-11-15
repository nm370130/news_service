# news-service
# A high-performance backend service for ingesting, searching, filtering, and summarizing news articles.
# uilt with Golang, Gin, PostgreSQL, Redis(* not added, but we can use as cache layer), Docker, and Ollama (local LLM).


# Features
1. Bulk Article Ingestion

Import multiple articles into PostgreSQL.

2. Full-Text Search & Filters

Search news using keywords or categories.

3. Trending Articles

Sorted by relevance & recency.

4. Nearby Search With Haversine

Find location-based articles using geospatial logic.

5. AI-Powered Summaries

Uses Ollama LLM (local) to generate article summaries.

6. Persisted Summaries

Summary is saved to DB for fast future responses.

7. Fully Dockerized

# Includes containers for:

PostgreSQL

Redis

Ollama

News Service API

# Project Structure
.
├── cmd/news-service/          # Main app
├── internal/
│   ├── api/                   # Gin handlers
│   ├── service/               # Business logic + LLM summary logic
│   ├── store/                 # PostgreSQL queries
│   └── llm/                   # Ollama LLM client
├── pkg/models/                # Article model
├── ingest/                    # Sample ingestion files
├── docker/                    # docker-compose.yml
├── docs/
│   ├── openapi.yaml           # OpenAPI docs
│   └── News Service.postman_collection.json
└── README.md


# Getting Started
Clone the repository
git clone 
cd news_service

Run With Docker (Recommended)

# Start everything:

docker compose -f docker/docker-compose.yml up --build -d

# Service	Port
  News API	   8080
  PostgreSQL	5432
  Redis	      6379
  Ollama	      11434

# Check containers:

docker ps

# LLM Setup (Ollama)

Check installed models:

curl http://localhost:11434/api/tags


# Install a tiny fast model:

curl -X POST http://localhost:11434/api/pull \
  -H "Content-Type: application/json" \
  -d '{"model":"smollm2:135m"}'


# Check running:

curl http://localhost:11434


# Expect:

Ollama is running

# Test the APIs
1. Ingest Articles
curl -X POST -H "Content-Type: application/json" \
  --data-binary @ingest/sample_articles.json \
  http://localhost:8080/v1/news/ingest

2. Search
GET /v1/news/search?q=golang&limit=10
Example:

curl "http://localhost:8080/v1/news/search?q=go&limit=5"

3. Category Filter
GET /v1/news/category?category=Sports

4. Trending
GET /v1/news/trending?limit=10

5. Nearby Search
GET /v1/news/nearby?lat=12.97&lon=77.59&radius=5&limit=10
Response includes:

{
  "meta": {
    "count": 2,
    "radius_km": 5
  },
  "data": [
    {
      "id": "...",
      "title": "...",
      "distance_km": 2.11
    }
  ]
}

6. Generate Summary (LLM)
POST /v1/news/{id}/summary

Example:

curl -X POST http://localhost:8080/v1/news/4f168b9a-8861-43d3-a1ac-b44a298910ea/summary


Response:

{
  "id": "4f168b9a-8861-43d3-a1ac-b44a298910ea",
  "summary": "A meetup about Golang happening..."
}


This summary is saved to DB.
Calling again returns instantly.

# API Documentation
# OpenAPI (Swagger)

Location:

docs/openapi.yaml


Run Swagger UI locally:

docker run --rm -p 8082:8080 \
  -e SWAGGER_JSON=/spec/openapi.yaml \
  -v $(pwd)/docs/openapi.yaml:/spec/openapi.yaml \
  swaggerapi/swagger-ui


Open browser:

http://localhost:8082

# Postman Collection

Import:

docs/News Service.postman_collection.json


# Architecture Overview
1. Handler Layer (Gin)
- Input validation
- Query parsing
- Response metadata
- Calls service methods

2. Service Layer
- Business logic
- LLM summarization
- Near-distance calculation
- Deduplication
- Default values

3. Store Layer (PostgreSQL)
- Typed TEXT[] support
- SQLx prepared statements
- Efficient indexes
- Safe transactions

4. LLM Client Layer
- Connects to Ollama
- Non-stream mode
- Extracts clean summary
- Saves to database

# Database Schema
CREATE TABLE articles(
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

CREATE INDEX idx_articles_published ON articles(published_at);
CREATE INDEX idx_articles_relevance ON articles(relevance_score);
CREATE INDEX idx_articles_categories ON articles USING GIN (categories);


# LLM Summary Pipeline
Flow:

1 Fetch article by ID
2 Build prompt
3 Call Ollama (POST /api/generate)
4 Extract final JSON line
5 Save summary into DB
6 Return clean summary

This avoids streaming garbage text.

🔍 Nearby SQL Optimization (optional upgrade)

Previously: in-memory filtering

currently Optimized SQL version:

SELECT *,
  (
    6371 * acos(
      cos(radians($1)) * cos(radians(latitude))
      * cos(radians(longitude) - radians($2))
      + sin(radians($1)) * sin(radians(latitude))
    )
  ) AS distance_km
FROM articles
HAVING distance_km <= $3
ORDER BY distance_km
LIMIT $4;

# Rebuild After Code Changes
docker compose -f docker/docker-compose.yml build --no-cache
docker compose -f docker/docker-compose.yml up -d

# news_service
Using LLM, creating/getting news data

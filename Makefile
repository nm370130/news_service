.PHONY: up build run ingest

up:
	docker compose -f docker/docker-compose.yml up --build

build:
	go build -o news-service ./cmd/news-service

run:
	go run ./cmd/news-service

ingest:
	curl -X POST -H "Content-Type: application/json" --data-binary @ingest/sample_articles.json http://localhost:8080/v1/news/ingest

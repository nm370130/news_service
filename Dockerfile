# Build for linux/arm64 (Apple Silicon)
FROM --platform=linux/arm64 golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN apk add --no-cache git build-base
RUN go env -w GOPROXY=https://proxy.golang.org,direct
RUN go mod download

COPY . .

# compile for linux/arm64 explicitly
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /app/news-service ./cmd/news-service

FROM --platform=linux/arm64 alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /app/news-service /app/news-service
EXPOSE 8080
ENTRYPOINT ["/app/news-service"]

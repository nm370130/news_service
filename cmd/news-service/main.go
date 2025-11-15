package main

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/gin-gonic/gin"
    _ "github.com/lib/pq"
    "github.com/nitesh/news_service/internal/api"
    "github.com/nitesh/news_service/internal/service"
    "github.com/nitesh/news_service/internal/store"
    "github.com/nitesh/news_service/internal/llm"
    "github.com/redis/go-redis/v9"
)

func envOrDefault(key, d string) string {
    v := os.Getenv(key)
    if v == "" {
        return d
    }
    return v
}

func main() {
    dbHost := envOrDefault("DB_HOST", "localhost")
    dbPort := envOrDefault("DB_PORT", "5432")
    dbName := envOrDefault("DB_NAME", "scout_db")
    dbUser := envOrDefault("DB_USER", "scout_user")
    dbPass := envOrDefault("DB_PASS", "Scout@1111")
    redisAddr := envOrDefault("REDIS_ADDR", "localhost:6379")
    port := envOrDefault("PORT", "8080")

    pgUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbPort, dbName)
    db, err := sql.Open("postgres", pgUrl)
    if err != nil {
        log.Fatalf("db open: %v", err)
    }
    // simple ping + wait (db might be starting in docker)
    for i := 0; i < 10; i++ {
        if err = db.Ping(); err == nil {
            break
        }
        log.Printf("waiting for db: attempt %d, err: %v", i+1, err)
        time.Sleep(2 * time.Second)
    }
    if err != nil {
        log.Fatalf("could not connect to db: %v", err)
    }

    // ensure tables exist (run migrations)
    if err := store.RunMigrations(db); err != nil {
        log.Fatalf("migrations: %v", err)
    }

    redisOpts := &redis.Options{Addr: redisAddr}
    rdb := redis.NewClient(redisOpts)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := rdb.Ping(ctx).Err(); err != nil {
        log.Printf("warning: redis ping failed: %v", err)
    }

    repo := store.NewPgStore(db)

     // create LLM client (reads LLM_URL, LLM_MODEL from env)
    llmClient := llm.NewClientFromEnv()

    svc := service.NewService(repo, rdb, llmClient)

    // svc := service.NewService(repo, rdb)
    handler := api.NewHandler(svc)

    router := gin.Default()
    api.RegisterRoutes(router, handler)

    log.Printf("listening on :%s", port)
    if err := router.Run(":" + port); err != nil {
        log.Fatalf("server failed: %v", err)
    }
}

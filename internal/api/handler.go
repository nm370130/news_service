package api

import (
	"context"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nitesh/news_service/internal/service"
	"github.com/nitesh/news_service/pkg/models"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func RegisterRoutes(r *gin.Engine, h *Handler) {
	v1 := r.Group("/v1")
	{
		v1.POST("/news/ingest", h.Ingest)
		v1.GET("/news/search", h.Search)
		v1.GET("/news/category", h.Category)
		v1.GET("/news/trending", h.Trending)
		v1.GET("/news/nearby", h.Nearby)
		v1.POST("/news/:id/summary", h.GenerateSummary)
	}
}

// Ingest: POST /v1/news/ingest
// Body: JSON array of articles
func (h *Handler) Ingest(c *gin.Context) {
	var payload []*models.Article
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json: " + err.Error()})
		return
	}
	ctx := context.Background()
	if err := h.svc.Ingest(ctx, payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ingest failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"meta": gin.H{"imported": len(payload)},
	})
}

// Search: GET /v1/news/search?q=...&limit=10
func (h *Handler) Search(c *gin.Context) {
	q := c.Query("q")
	lim := parseLimit(c.DefaultQuery("limit", "10"))
	ctx := context.Background()
	res, err := h.svc.Search(ctx, q, lim)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"meta": gin.H{
			"query": q,
			"count": len(res),
			"limit": lim,
		},
		"data": res,
	})
}

// Category: GET /v1/news/category?category=Technology&limit=10
func (h *Handler) Category(c *gin.Context) {
	category := c.Query("category")
	if category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing category parameter"})
		return
	}
	lim := parseLimit(c.DefaultQuery("limit", "10"))
	ctx := context.Background()
	res, err := h.svc.Category(ctx, category, lim)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"meta": gin.H{
			"category": category,
			"count":    len(res),
			"limit":    lim,
		},
		"data": res,
	})
}

// Trending: GET /v1/news/trending?limit=10
func (h *Handler) Trending(c *gin.Context) {
	lim := parseLimit(c.DefaultQuery("limit", "10"))
	ctx := context.Background()
	res, err := h.svc.Trending(ctx, lim)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"meta": gin.H{
			"count": len(res),
			"limit": lim,
		},
		"data": res,
	})
}

// Nearby: GET /v1/news/nearby?lat=12.97&lon=77.59&radius=10&limit=20
func (h *Handler) Nearby(c *gin.Context) {
	q := c.Request.URL.Query()

	lat, latErr := strconv.ParseFloat(q.Get("lat"), 64)
	lon, lonErr := strconv.ParseFloat(q.Get("lon"), 64)
	radius, radiusErr := strconv.ParseFloat(q.Get("radius"), 64)
	limit := parseLimit(c.DefaultQuery("limit", "20"))

	// Basic validation
	if latErr != nil || lonErr != nil || radiusErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or missing lat/lon/radius parameters"})
		return
	}
	if math.Abs(lat) > 90 || math.Abs(lon) > 180 || radius <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lat/lon/radius values"})
		return
	}

	results, err := h.svc.Nearby(c.Request.Context(), lat, lon, radius, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"meta": gin.H{
			"count":     len(results),
			"radius_km": radius,
			"limit":     limit,
		},
		"data": results,
	})
}

// GenerateSummary: POST /v1/news/:id/summary
// Triggers LLM summarization, saves summary to DB and returns it.
func (h *Handler) GenerateSummary(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id parameter"})
		return
	}
	ctx := c.Request.Context()

	summary, err := h.svc.SummarizeArticle(ctx, id)
	if err != nil {
		// map known errors to proper status codes if you want (e.g., not found)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      id,
		"summary": summary,
	})
}

// parseLimit ensures a sane integer limit, with bounds
func parseLimit(s string) int {
	l, err := strconv.Atoi(s)
	if err != nil || l <= 0 {
		return 10
	}
	if l > 200 {
		return 200
	}
	return l
}

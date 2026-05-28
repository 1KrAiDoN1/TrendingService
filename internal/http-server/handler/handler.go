package handler

import (
	"net/http"
	"strconv"
	"time"
	"trendservice/internal/domain"
	"trendservice/internal/metrics"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers содержит все HTTP обработчики.
type Handlers struct {
	aggregator AggregatorService
	stoplist   StoplistService
	defaultN   int
	maxN       int
	log        *zap.Logger
}

// New создает новый экземпляр Handlers.
func New(aggregator AggregatorService, stoplist StoplistService, defaultN, maxN int, log *zap.Logger) *Handlers {
	return &Handlers{
		aggregator: aggregator,
		stoplist:   stoplist,
		defaultN:   defaultN,
		maxN:       maxN,
		log:        log,
	}
}

func (h *Handlers) HandleTop(c *gin.Context) {
	metrics.TopRequests.Inc()
	start := time.Now()
	defer func() { metrics.TopLatency.Observe(time.Since(start).Seconds()) }()

	// Парсим параметр n
	n := h.defaultN
	if nStr := c.Query("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			if parsed > h.maxN {
				parsed = h.maxN
			}
			n = parsed
		}
	}

	ctx := c.Request.Context()
	snap := h.aggregator.Snapshot()

	// Фильтруем стоп-лист
	out := make([]domain.TopEntry, 0, n)
	for _, e := range snap.Entries {
		if h.stoplist.Has(ctx, e.Query) {
			continue
		}
		out = append(out, e)
		if len(out) >= n {
			break
		}
	}

	// Edge-cache на 1 секунду — снижает RPS до сервиса под пиком
	c.Header("Cache-Control", "public, max-age=1")

	c.JSON(http.StatusOK, gin.H{
		"window_seconds": snap.WindowSec,
		"generated_at":   snap.GeneratedAt,
		"items":          out,
	})
}

type stoplistReq struct {
	Add    []string `json:"add" binding:"omitempty"`
	Remove []string `json:"remove" binding:"omitempty"`
}

func (h *Handlers) HandleGetStoplist(c *gin.Context) {
	ctx := c.Request.Context()

	items, err := h.stoplist.List(ctx)
	if err != nil {
		h.log.Error("failed to list stoplist", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handlers) HandleUpdateStoplist(c *gin.Context) {
	ctx := c.Request.Context()

	var req stoplistReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad json"})
		return
	}

	if len(req.Add) > 0 {
		if err := h.stoplist.Add(ctx, req.Add...); err != nil {
			h.log.Error("failed to add to stoplist", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
	}

	if len(req.Remove) > 0 {
		if err := h.stoplist.Remove(ctx, req.Remove...); err != nil {
			h.log.Error("failed to remove from stoplist", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
	}

	c.Status(http.StatusNoContent)
}

func (h *Handlers) HandleHealth(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}

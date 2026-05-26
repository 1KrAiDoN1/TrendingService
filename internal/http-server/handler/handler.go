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

// HandleTop обрабатывает запросы на получение топа популярных запросов.
// @Summary Получить топ популярных запросов
// @Description Возвращает топ-N самых популярных поисковых запросов за последние 5 минут
// @Tags trends
// @Accept json
// @Produce json
// @Param n query int false "Количество запросов в топе" default(10) minimum(1) maximum(100)
// @Success 200 {object} map[string]interface{}
// @Router /top [get]
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

// HandleGetStoplist возвращает текущий стоп-лист.
// @Summary Получить стоп-лист
// @Description Возвращает список всех слов в стоп-листе
// @Tags stoplist
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /stoplist [get]
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

// HandleUpdateStoplist обновляет стоп-лист.
// @Summary Обновить стоп-лист
// @Description Добавляет или удаляет слова из стоп-листа
// @Tags stoplist
// @Accept json
// @Produce json
// @Param request body stoplistReq true "Слова для добавления/удаления"
// @Success 204
// @Router /stoplist [post]
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

// HandleHealth обрабатывает health check запросы.
// @Summary Health check
// @Description Проверка работоспособности сервиса
// @Tags health
// @Produce plain
// @Success 200 {string} string "ok"
// @Router /healthz [get]
func (h *Handlers) HandleHealth(c *gin.Context) {
	c.String(http.StatusOK, "ok")
}


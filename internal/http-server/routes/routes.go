package routes

import (
	"trendservice/internal/http-server/handler"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SetupRoutes настраивает маршруты HTTP сервера.
func SetupRoutes(router *gin.RouterGroup, handlers *handler.Handlers) {
	router.GET("/top", handlers.HandleTop)
	router.GET("/stoplist", handlers.HandleGetStoplist)
	router.POST("/stoplist", handlers.HandleUpdateStoplist)
	router.GET("/healthz", handlers.HandleHealth)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"trendservice/internal/config"
	"trendservice/internal/http-server/handler"
	"trendservice/internal/http-server/routes"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Server представляет HTTP сервер для API трендов.
type Server struct {
	server   *http.Server
	config   config.ServiceConfig
	router   *gin.Engine
	handlers *handler.Handlers
	logger   *zap.Logger
}

// New создает новый HTTP сервер.
func New(
	cfg config.ServiceConfig,
	aggregator handler.AggregatorService,
	stoplist handler.StoplistService,
	log *zap.Logger,
) *Server {
	// Устанавливаем режим gin
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger(log))

	handlers := handler.New(
		aggregator,
		stoplist,
		cfg.Server.DefaultTopN,
		cfg.Server.MaxTopN,
		log,
	)

	server := &http.Server{
		Addr:         cfg.Server.HTTPAddr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return &Server{
		server:   server,
		config:   cfg,
		router:   router,
		handlers: handlers,
		logger:   log,
	}
}

func (s *Server) setupRoutes() {
	router := s.router.Group("")
	routes.SetupRoutes(router, s.handlers)
}

func (s *Server) Run() error {
	s.setupRoutes()
	s.logger.Info("starting HTTP server", zap.String("address", s.config.Server.HTTPAddr))

	if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Shutdown выполняет graceful shutdown HTTP сервера.
func (s *Server) Shutdown(ctx context.Context) error {
	ctx_shutdown, cancel := context.WithTimeout(ctx, s.config.Server.ShutdownTimeout)
	defer cancel()

	s.logger.Info("shutting down HTTP server...")
	if err := s.server.Shutdown(ctx_shutdown); err != nil {
		s.logger.Error("server forced to shutdown", zap.Error(err))
		return fmt.Errorf("server shutdown failed: %w", err)
	}
	s.logger.Info("http server gracefully shut down")
	return nil
}

// ginLogger создает middleware для логирования запросов через zap.
func ginLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Обрабатываем запрос
		c.Next()

		// Логируем после обработки
		if c.Request.URL.Path != "/healthz" && c.Request.URL.Path != "/metrics" {
			log.Info("http request",
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.Int("status", c.Writer.Status()),
				zap.String("ip", c.ClientIP()),
			)
		}
	}
}

// ServeHTTP реализует интерфейс http.Handler для тестирования.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"trendservice/internal/broker/consumer/kafka"
	"trendservice/internal/config"
	httpserver "trendservice/internal/http-server"
	"trendservice/internal/repository/cache/redis"
	aggregatoradapter "trendservice/internal/usecase/aggregator/adapter"
	"trendservice/internal/usecase/stoplist"
	"trendservice/pkg/lib/logger/zaplogger"

	"go.uber.org/zap"
)

// Run запускает приложение.
func Run(ctx context.Context, log *zap.Logger, cfg config.ServiceConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Канал для получения сигналов завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("starting trend service",
		zap.String("http_addr", cfg.Server.HTTPAddr),
		zap.Int("window_seconds", cfg.Server.WindowSeconds),
		zap.Int("shards", cfg.Server.Shards),
	)

	// Инициализируем Redis клиент
	redisClient, err := redis.NewClient(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
		log,
	)
	if err != nil {
		log.Error("failed to connect to redis", zap.Error(err))
		return fmt.Errorf("redis connection failed: %w", err)
	}
	log.Info("connected to redis", zap.String("addr", cfg.Redis.Addr))

	defer func() {
		log.Info("closing redis connection...")
		if err := redisClient.Close(); err != nil {
			log.Error("failed to close redis", zap.Error(err))
		}
		log.Info("redis connection closed")
	}()

	// Инициализируем стоп-лист
	stoplistService := stoplist.New(redisClient, log)

	// Инициализируем агрегатор
	agg := aggregatoradapter.New(
		cfg.Server.WindowSeconds,
		cfg.Server.Shards,
		cfg.Server.MaxTopN*3, // запас под фильтрацию стоп-листом
		cfg.Server.DedupTTL,
	)

	// Запускаем фоновую задачу агрегатора (пересчет снапшотов)
	go agg.Run(ctx, cfg.Server.SnapshotInterval)

	// Инициализируем Kafka консьюмер
	kafkaConsumer := kafka.NewKafkaConsumer(
		kafka.Config{
			Brokers:   cfg.Broker.Brokers,
			Topic:     cfg.Broker.Topic,
			GroupID:   cfg.Broker.GroupID,
			MaxSkew:   cfg.Server.MaxClockSkew,
			WindowSec: cfg.Server.WindowSeconds,
			Workers:   cfg.Server.WorkerCount,
		},
		agg,
		log,
	)

	defer func() {
		log.Info("closing kafka consumer...")
		if err := kafkaConsumer.Close(); err != nil {
			log.Error("failed to close kafka consumer", zap.Error(err))
		}
		log.Info("kafka consumer closed")
	}()

	// Запускаем Kafka консьюмер в горутине
	consumerDone := make(chan error, 1)
	go func() {
		log.Info("starting kafka consumer...")
		if err := kafkaConsumer.Run(ctx); err != nil {
			consumerDone <- err
		}
		close(consumerDone)
	}()

	// Инициализируем HTTP сервер
	server := httpserver.New(
		cfg,
		agg,
		stoplistService,
		log,
	)

	// Запускаем HTTP сервер в горутине
	serverDone := make(chan error, 1)
	go func() {
		log.Info("starting HTTP server...")
		if err := server.Run(); err != nil {
			serverDone <- err
		}
		close(serverDone)
	}()

	// Ожидаем сигнала завершения или ошибки
	select {
	case sig := <-sigChan:
		log.Info("received shutdown signal", zap.String("signal", sig.String()))
		cancel()

		// Graceful shutdown HTTP сервера
		if err := server.Shutdown(ctx); err != nil {
			log.Error("failed to shutdown HTTP server", zap.Error(err))
		}

		log.Info("waiting for goroutines to finish...")
		log.Info("application gracefully shut down")
		return nil

	case err := <-serverDone:
		if err != nil {
			log.Error("http server stopped with error", zap.Error(err))
			cancel()
			return err
		}
		return nil

	case err := <-consumerDone:
		if err != nil {
			log.Error("kafka consumer stopped with error", zap.Error(err))
			cancel()
			return err
		}
		return nil
	}
}

// Start - точка входа для запуска приложения.
func Start() error {
	// Загружаем конфигурацию
	cfg, err := config.LoadServiceConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Инициализируем логгер
	log := zaplogger.SetupLogger()
	defer func() {
		if err := log.Sync(); err != nil {
			// Игнорируем ошибки sync для stderr/stdout
			if err.Error() != "sync /dev/stderr: inappropriate ioctl for device" &&
				err.Error() != "sync /dev/stdout: inappropriate ioctl for device" {
				fmt.Printf("failed to sync logger: %v\n", err)
			}
		}
	}()

	// Запускаем приложение
	ctx := context.Background()
	return Run(ctx, log, cfg)
}

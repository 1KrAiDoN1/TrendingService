// Package kafka реализует Consumer интерфейс для Apache Kafka.
package kafka

import (
	"context"
	"encoding/json"
	"time"
	"trendservice/internal/aggregator"
	"trendservice/internal/consumer"
	"trendservice/internal/domain"
	"trendservice/internal/metrics"
	"trendservice/pkg/contract"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// Проверка соответствия интерфейсу на этапе компиляции.
var _ consumer.Consumer = (*KafkaConsumer)(nil)

// KafkaConsumer реализует чтение событий из Kafka.
type KafkaConsumer struct {
	reader       *kafka.Reader
	agg          aggregator.Aggregator
	maxSkew      time.Duration
	windowSec    int
	workersCount int
	log          *zap.Logger
}

// Config содержит настройки для создания Kafka консьюмера.
type Config struct {
	Brokers   []string
	Topic     string
	GroupID   string
	MaxSkew   time.Duration
	WindowSec int
	Workers   int
}

// NewKafkaConsumer создает новый Kafka консьюмер.
func NewKafkaConsumer(cfg Config, agg aggregator.Aggregator, log *zap.Logger) *KafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		GroupID:        cfg.GroupID,
		Topic:          cfg.Topic,
		MinBytes:       10e3,
		MaxBytes:       10e6,
		CommitInterval: time.Second, // at-least-once, для трендов потеря/дубль некритичны
		StartOffset:    kafka.LastOffset,
	})

	log.Info("kafka consumer created",
		zap.Strings("brokers", cfg.Brokers),
		zap.String("topic", cfg.Topic),
		zap.String("group_id", cfg.GroupID),
		zap.Int("workers", cfg.Workers),
	)

	return &KafkaConsumer{
		reader:       reader,
		agg:          agg,
		maxSkew:      cfg.MaxSkew,
		windowSec:    cfg.WindowSec,
		workersCount: cfg.Workers,
		log:          log,
	}
}

// Run запускает процесс чтения сообщений из Kafka.
// Блокирует выполнение до отмены контекста или критической ошибки.
func (c *KafkaConsumer) Run(ctx context.Context) error {
	msgCh := make(chan kafka.Message, c.workersCount)

	// Запускаем пул воркеров для обработки сообщений
	for i := 1; i <= c.workersCount; i++ {
		go c.worker(ctx, msgCh, i)
	}

	c.log.Info("kafka consumer started", zap.Int("workers", c.workersCount))

	defer close(msgCh)

	for {
		m, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.log.Info("kafka consumer stopped by context")
				return nil
			}
			c.log.Error("kafka read error", zap.Error(err))
			continue
		}

		select {
		case msgCh <- m:
		case <-ctx.Done():
			c.log.Info("kafka consumer stopped by context")
			return nil
		}
	}
}

// worker обрабатывает сообщения из канала.
func (c *KafkaConsumer) worker(ctx context.Context, ch <-chan kafka.Message, workerID int) {
	maxSkewSec := int64(c.maxSkew.Seconds())

	c.log.Debug("kafka worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-ctx.Done():
			c.log.Debug("kafka worker stopped", zap.Int("worker_id", workerID))
			return
		case m, ok := <-ch:
			if !ok {
				c.log.Debug("kafka worker channel closed", zap.Int("worker_id", workerID))
				return
			}

			metrics.EventsConsumed.Inc()

			// Десериализация события
			var contractEvent contract.SearchEvent
			if err := json.Unmarshal(m.Value, &contractEvent); err != nil {
				metrics.EventsDropped.WithLabelValues("bad_json").Inc()
				c.log.Warn("failed to unmarshal event", zap.Error(err))
				continue
			}

			// Валидация
			if contractEvent.Query == "" {
				metrics.EventsDropped.WithLabelValues("empty_query").Inc()
				continue
			}

			// Конвертация в domain модель
			event := c.contractToDomain(contractEvent)

			// Определяем timestamp
			ts := event.Timestamp.Unix()
			if ts == 0 {
				ts = time.Now().Unix()
			}

			// Добавляем в агрегатор
			dedupKey := event.GetDeduplicationKey()
			if c.agg.Add(event.Query, dedupKey, ts, time.Now().Unix(), maxSkewSec) {
				metrics.EventsAccepted.Inc()
			} else {
				metrics.EventsDropped.WithLabelValues("dedup_or_window").Inc()
			}
		}
	}
}

// contractToDomain конвертирует contract.SearchEvent в domain.SearchEvent.
func (c *KafkaConsumer) contractToDomain(ce contract.SearchEvent) domain.SearchEvent {
	return domain.SearchEvent{
		Query:     ce.Query,
		UserID:    ce.UserID,
		SessionID: ce.SessionID,
		RequestID: ce.RequestID,
		Timestamp: time.Unix(ce.Timestamp, 0),
	}
}

// Close освобождает ресурсы консьюмера.
func (c *KafkaConsumer) Close() error {
	c.log.Info("closing kafka consumer")
	return c.reader.Close()
}

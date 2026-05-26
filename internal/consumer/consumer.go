package consumer

import "context"

// Consumer определяет интерфейс для чтения событий из брокера сообщений.
// Это позволяет легко заменить Kafka на другой брокер (NATS, RabbitMQ и т.д.).
type Consumer interface {
	// Run запускает процесс чтения сообщений из брокера.
	// Блокирует выполнение до отмены контекста или критической ошибки.
	Run(ctx context.Context) error

	// Close освобождает ресурсы консьюмера.
	Close() error
}

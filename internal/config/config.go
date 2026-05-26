package config

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/ilyakaznacheev/cleanenv"
)

// LoadServiceConfig загружает конфигурацию из .env файла или переменных окружения.
func LoadServiceConfig() (ServiceConfig, error) {
	var serviceConfig ServiceConfig

	if err := cleanenv.ReadConfig(".env", &serviceConfig); err != nil {
		if err := cleanenv.ReadEnv(&serviceConfig); err != nil {
			return ServiceConfig{}, fmt.Errorf("failed to read environment variables: %w", err)
		}
	}
	if err := validator.New().Struct(&serviceConfig); err != nil {
		return serviceConfig, fmt.Errorf("config validation failed: %w", err)
	}

	return serviceConfig, nil
}

// ServiceConfig содержит всю конфигурацию сервиса.
type ServiceConfig struct {
	Server ServerConfig
	Broker BrokerConfig
	Redis  RedisConfig
}

// ServerConfig содержит настройки HTTP сервера и агрегатора.
type ServerConfig struct {
	HTTPAddr         string        `env:"SERVER_ADDRESS" env-default:":8080" validate:"required"`
	ReadTimeout      time.Duration `env:"SERVER_READ_TIMEOUT" env-default:"15s" validate:"required"`
	WriteTimeout     time.Duration `env:"SERVER_WRITE_TIMEOUT" env-default:"15s" validate:"required"`
	IdleTimeout      time.Duration `env:"SERVER_IDLE_TIMEOUT" env-default:"120s" validate:"required"`
	ShutdownTimeout  time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" env-default:"30s" validate:"required"`
	WindowSeconds    int           `env:"WINDOW_SECONDS" env-default:"300" validate:"required,min=60"`
	SnapshotInterval time.Duration `env:"SNAPSHOT_INTERVAL" env-default:"500ms" validate:"required"`
	DefaultTopN      int           `env:"DEFAULT_TOP_N" env-default:"10" validate:"required,min=1"`
	MaxTopN          int           `env:"MAX_TOP_N" env-default:"100" validate:"required,min=1"`
	DedupTTL         time.Duration `env:"DEDUP_TTL" env-default:"10s" validate:"required"`
	Shards           int           `env:"SHARDS" env-default:"16" validate:"required,min=1"`
	WorkerCount      int           `env:"WORKER_COUNT" env-default:"4" validate:"required,min=1"`
	MaxClockSkew     time.Duration `env:"MAX_CLOCK_SKEW" env-default:"60s" validate:"required"`
}

// BrokerConfig содержит настройки Kafka брокера.
type BrokerConfig struct {
	Brokers []string `env:"BROKERS" env-default:"localhost:9092" validate:"required"`
	Topic   string   `env:"TOPIC" env-default:"search-queries" validate:"required"`
	GroupID string   `env:"GROUP_ID" env-default:"trend-service-group" validate:"required"`
}

// RedisConfig содержит настройки Redis клиента.
type RedisConfig struct {
	Addr     string `env:"REDIS_ADDR" env-default:"localhost:6379" validate:"required"`
	Password string `env:"REDIS_PASSWORD" env-default:""`
	DB       int    `env:"REDIS_DB" env-default:"0" validate:"min=0"`
}

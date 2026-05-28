package cache

import (
	"context"
	"time"
)

// CacheClient определяет интерфейс для работы с кешем/хранилищем.
// Используется для реализации стоп-листа через Redis или другое хранилище.
type CacheClient interface {
	// Set устанавливает значение по ключу с TTL.
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Get получает значение по ключу.
	Get(ctx context.Context, key string) (string, error)

	// Del удаляет ключ(и).
	Del(ctx context.Context, keys ...string) error

	// SAdd добавляет элементы в множество.
	SAdd(ctx context.Context, key string, members ...interface{}) error

	// SRem удаляет элементы из множества.
	SRem(ctx context.Context, key string, members ...interface{}) error

	// SMembers возвращает все элементы множества.
	SMembers(ctx context.Context, key string) ([]string, error)

	// SIsMember проверяет наличие элемента в множестве.
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)

	// Ping проверяет соединение с хранилищем.
	Ping(ctx context.Context) error

	// Close закрывает соединение.
	Close() error
}

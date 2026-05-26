package handler

import (
	"context"
	"trendservice/internal/domain"
)

// AggregatorService определяет интерфейс для работы с агрегатором трендов.
type AggregatorService interface {
	// Snapshot возвращает текущий снимок топа запросов.
	Snapshot() *domain.TopSnapshot
}

// StoplistService определяет интерфейс для работы со стоп-листом.
type StoplistService interface {
	// Has проверяет, находится ли запрос в стоп-листе.
	Has(ctx context.Context, query string) bool

	// Add добавляет слова в стоп-лист.
	Add(ctx context.Context, words ...string) error

	// Remove удаляет слова из стоп-листа.
	Remove(ctx context.Context, words ...string) error

	// List возвращает все слова из стоп-листа.
	List(ctx context.Context) ([]string, error)
}

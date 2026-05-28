// Package stoplist — стоп-лист с возможностью обновления "на лету".
// Использует Redis для персистентного хранения и синхронизации между инстансами.
package stoplist

import (
	"context"
	"trendservice/internal/repository/cache"
	aggregatoradapter "trendservice/internal/usecase/aggregator/adapter"

	"go.uber.org/zap"
)

const stoplistKey = "trending:stoplist"

// internal/stoplist/stoplist.go
//
//go:generate mockery --name Stoplist --dir internal/usecase/stoplist --output internal/usecase/mocks
type Stoplist interface {
	// Has проверяет, находится ли запрос в стоп-листе.
	Has(ctx context.Context, query string) bool

	// Add добавляет слова в стоп-лист.
	Add(ctx context.Context, words ...string) error

	// Remove удаляет слова из стоп-листа.
	Remove(ctx context.Context, words ...string) error

	// List возвращает все слова из стоп-листа.
	List(ctx context.Context) ([]string, error)
}

// stoplist управляет списком нежелательных запросов через Redis.
type stoplist struct {
	cache cache.CacheClient
	log   *zap.Logger
}

// New создает новый стоп-лист с использованием Redis.
func New(cache cache.CacheClient, log *zap.Logger) Stoplist {
	return &stoplist{
		cache: cache,
		log:   log,
	}
}

// Has проверяет, находится ли запрос в стоп-листе.
func (s *stoplist) Has(ctx context.Context, query string) bool {
	normalized := aggregatoradapter.Normalize(query)
	if normalized == "" {
		return false
	}

	exists, err := s.cache.SIsMember(ctx, stoplistKey, normalized)
	if err != nil {
		s.log.Error("failed to check stoplist", zap.Error(err), zap.String("query", normalized))
		return false
	}

	return exists
}

// Add добавляет слова в стоп-лист.
func (s *stoplist) Add(ctx context.Context, words ...string) error {
	if len(words) == 0 {
		return nil
	}

	normalized := make([]interface{}, 0, len(words))
	for _, w := range words {
		if n := aggregatoradapter.Normalize(w); n != "" {
			normalized = append(normalized, n)
		}
	}

	if len(normalized) == 0 {
		return nil
	}

	if err := s.cache.SAdd(ctx, stoplistKey, normalized...); err != nil {
		s.log.Error("failed to add to stoplist", zap.Error(err), zap.Int("count", len(normalized)))
		return err
	}

	s.log.Info("added to stoplist", zap.Int("count", len(normalized)))
	return nil
}

// Remove удаляет слова из стоп-листа.
func (s *stoplist) Remove(ctx context.Context, words ...string) error {
	if len(words) == 0 {
		return nil
	}

	normalized := make([]interface{}, 0, len(words))
	for _, w := range words {
		if n := aggregatoradapter.Normalize(w); n != "" {
			normalized = append(normalized, n)
		}
	}

	if len(normalized) == 0 {
		return nil
	}

	if err := s.cache.SRem(ctx, stoplistKey, normalized...); err != nil {
		s.log.Error("failed to remove from stoplist", zap.Error(err), zap.Int("count", len(normalized)))
		return err
	}

	s.log.Info("removed from stoplist", zap.Int("count", len(normalized)))
	return nil
}

// List возвращает все слова из стоп-листа.
func (s *stoplist) List(ctx context.Context) ([]string, error) {
	members, err := s.cache.SMembers(ctx, stoplistKey)
	if err != nil {
		s.log.Error("failed to list stoplist", zap.Error(err))
		return nil, err
	}

	return members, nil
}

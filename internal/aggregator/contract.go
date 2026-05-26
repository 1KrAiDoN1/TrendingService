package aggregator

import (
	"context"
	"time"
	"trendservice/internal/domain"
)

type Aggregator interface {
	// Add учитывает событие поискового запроса.
	Add(query, userID string, eventTsUnix, nowUnix int64, maxSkew int64) bool

	// Snapshot возвращает текущий снимок топа.
	Snapshot() *domain.TopSnapshot

	// Run запускает фоновые задачи агрегатора.
	Run(ctx context.Context, snapshotInterval time.Duration)
}

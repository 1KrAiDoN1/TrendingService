package aggregatoradapter

import "trendservice/internal/domain"

// TopEntry — одна позиция в топе.
// Используем domain тип для согласованности.
type TopEntry = domain.TopEntry

// Snapshot — иммутабельная выборка топа на момент времени.
// Хранит больше элементов, чем спрашивают, чтобы стоп-лист не выкосил весь top.
// Используем domain тип для согласованности.
type Snapshot = domain.TopSnapshot

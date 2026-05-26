package aggregatoradapter

import (
	"container/heap"
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"trendservice/internal/domain"
	"unicode"
)

// aggregator — потокобезопасный агрегатор скользящего окна.
//
// Архитектурные решения:
//  1. Окно разбито на N бакетов по 1 секунде. На запись лочим только текущий бакет шарда.
//  2. Шардирование по hash(query) — снижает contention.
//  3. Снапшот топа пересчитывается фоновым воркером раз в SnapshotInterval
//     и публикуется через atomic.Pointer — чтение становится lock-free.
//  4. Стоп-лист применяется на чтении (см. api): чтобы изменения "на лету" работали и
//     для уже накопленных данных.
type aggregator struct {
	windowSec int
	shards    []*shard
	shardMask uint64
	dedup     *dedupCache
	snapshotN int // сколько элементов класть в снапшот (с запасом)
	current   atomic.Pointer[domain.TopSnapshot]
}

type shard struct {
	mu      sync.Mutex
	buckets []map[string]int64 // ring buffer, длина = windowSec
	head    int                // индекс текущего бакета
	headSec int64              // unix-секунда текущего бакета
}

// New создаёт агрегатор. dedupTTL=0 отключает дедуп.
func New(windowSec, shards, snapshotN int, dedupTTL time.Duration) *aggregator {
	n := 1
	for n < shards {
		n <<= 1
	}
	a := &aggregator{
		windowSec: windowSec,
		shards:    make([]*shard, n),
		shardMask: uint64(n - 1),
		snapshotN: snapshotN,
	}
	for i := range a.shards {
		s := &shard{
			buckets: make([]map[string]int64, windowSec),
		}
		for j := range s.buckets {
			s.buckets[j] = make(map[string]int64)
		}
		a.shards[i] = s
	}
	if dedupTTL > 0 {
		a.dedup = newDedupCache(64, dedupTTL)
	}
	// публикуем пустой снапшот, чтобы избежать nil
	a.current.Store(&domain.TopSnapshot{Entries: []domain.TopEntry{}, WindowSec: windowSec})
	return a
}

// Normalize — публичная нормализация, использует тот же алгоритм, что и Add.
// Применяется в стоп-листе для согласованности.
func Normalize(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	// collapse whitespace
	var b strings.Builder
	b.Grow(len(q))
	prevSpace := false
	for _, r := range q {
		if unicode.IsSpace(r) {
			if !prevSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			prevSpace = true
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	s := b.String()
	return strings.TrimRight(s, " ")
}

// Add учитывает событие. nowUnix — текущее время в секундах.
// Если event пришёл из прошлого старше окна или из будущего — игнорируется.
func (a *aggregator) Add(query, userID string, eventTsUnix, nowUnix int64, maxSkew int64) bool {
	if eventTsUnix > nowUnix+maxSkew || nowUnix-eventTsUnix > int64(a.windowSec) {
		return false
	}
	q := Normalize(query)
	if q == "" {
		return false
	}
	if a.dedup != nil && !a.dedup.shouldCount(userID, q, nowUnix) {
		return false
	}
	sh := a.shards[fnv64(q)&a.shardMask]
	sh.mu.Lock()
	sh.advance(nowUnix, a.windowSec)
	sh.buckets[sh.head][q]++
	sh.mu.Unlock()
	return true
}

// advance прокручивает кольцевой буфер до текущей секунды, очищая просроченные бакеты.
// Вызывается под мьютексом.
func (s *shard) advance(nowSec int64, windowSec int) {
	if s.headSec == 0 {
		s.headSec = nowSec
		return
	}
	diff := nowSec - s.headSec
	if diff <= 0 {
		return
	}
	if diff >= int64(windowSec) {
		// прошло больше окна — чистим всё
		for i := range s.buckets {
			if len(s.buckets[i]) > 0 {
				s.buckets[i] = make(map[string]int64)
			}
		}
		s.head = 0
		s.headSec = nowSec
		return
	}
	for i := int64(0); i < diff; i++ {
		s.head = (s.head + 1) % windowSec
		if len(s.buckets[s.head]) > 0 {
			s.buckets[s.head] = make(map[string]int64)
		}
	}
	s.headSec = nowSec
}

// Run запускает фоновые задачи: пересчёт снапшота и GC дедупа.
func (a *aggregator) Run(ctx context.Context, snapshotInterval time.Duration) {
	snapTicker := time.NewTicker(snapshotInterval)
	gcTicker := time.NewTicker(5 * time.Second)
	defer snapTicker.Stop()
	defer gcTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-snapTicker.C:
			a.rebuildSnapshot()
		case <-gcTicker.C:
			if a.dedup != nil {
				a.dedup.gc(time.Now().Unix())
			}
		}
	}
}

// rebuildSnapshot агрегирует все шарды и строит топ-N через min-heap.
func (a *aggregator) rebuildSnapshot() {
	now := time.Now().Unix()
	total := make(map[string]int64, 4096)
	for _, sh := range a.shards {
		sh.mu.Lock()
		sh.advance(now, a.windowSec)
		for _, b := range sh.buckets {
			for k, v := range b {
				total[k] += v
			}
		}
		sh.mu.Unlock()
	}
	entries := topN(total, a.snapshotN)
	a.current.Store(&domain.TopSnapshot{
		Entries:     entries,
		GeneratedAt: time.Now().UnixNano(),
		WindowSec:   a.windowSec,
	})
}

// Snapshot возвращает текущий опубликованный снапшот (lock-free).
func (a *aggregator) Snapshot() *domain.TopSnapshot {
	return a.current.Load()
}

// --- min-heap для top-N ---

type minHeap []domain.TopEntry

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i].Count < h[j].Count }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x any)        { *h = append(*h, x.(domain.TopEntry)) }
func (h *minHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func topN(m map[string]int64, n int) []domain.TopEntry {
	if n <= 0 || len(m) == 0 {
		return []domain.TopEntry{}
	}
	h := &minHeap{}
	heap.Init(h)
	for k, v := range m {
		if h.Len() < n {
			heap.Push(h, domain.TopEntry{Query: k, Count: v})
		} else if (*h)[0].Count < v {
			(*h)[0] = domain.TopEntry{Query: k, Count: v}
			heap.Fix(h, 0)
		}
	}
	res := make([]domain.TopEntry, h.Len())
	for i := h.Len() - 1; i >= 0; i-- {
		res[i] = heap.Pop(h).(domain.TopEntry)
	}
	return res
}

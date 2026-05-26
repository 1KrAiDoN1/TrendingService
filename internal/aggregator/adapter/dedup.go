package aggregatoradapter

import (
	"sync"
	"time"
)

// dedupCache — простой шардированный кэш с TTL для отсечки накруток.
// Ключ: userID + "|" + query. Значение: время последнего вхождения.
// Когда нет userID — дедуп не применяется (анонимы считаются всегда).
type dedupCache struct {
	shards []*dedupShard
	ttl    time.Duration
	mask   uint64
}

type dedupShard struct {
	mu sync.Mutex
	m  map[string]int64
}

func newDedupCache(shards int, ttl time.Duration) *dedupCache {
	// shards округлим до степени двойки
	n := 1
	for n < shards {
		n <<= 1
	}
	c := &dedupCache{
		shards: make([]*dedupShard, n),
		ttl:    ttl,
		mask:   uint64(n - 1),
	}
	for i := range c.shards {
		c.shards[i] = &dedupShard{m: make(map[string]int64, 1024)}
	}
	return c
}

// shouldCount возвращает true, если событие не является дубликатом.
func (c *dedupCache) shouldCount(userID, query string, nowUnix int64) bool {
	if userID == "" {
		return true
	}
	key := userID + "|" + query
	sh := c.shards[fnv64(key)&c.mask]
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if last, ok := sh.m[key]; ok && nowUnix-last < int64(c.ttl.Seconds()) {
		return false
	}
	sh.m[key] = nowUnix
	return true
}

// gc удаляет просроченные записи. Запускается периодически.
func (c *dedupCache) gc(nowUnix int64) {
	cutoff := nowUnix - int64(c.ttl.Seconds())
	for _, sh := range c.shards {
		sh.mu.Lock()
		for k, ts := range sh.m {
			if ts < cutoff {
				delete(sh.m, k)
			}
		}
		sh.mu.Unlock()
	}
}

// fnv64 — быстрый хэш без аллокаций.
func fnv64(s string) uint64 {
	const (
		offset = 1469598103934665603
		prime  = 1099511628211
	)
	var h uint64 = offset
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime
	}
	return h
}

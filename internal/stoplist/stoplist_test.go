package stoplist_test

import (
	"context"
	"testing"
	"time"
	"trendservice/internal/stoplist"
	"trendservice/pkg/lib/logger/zaplogger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCacheClient реализует интерфейс cache.CacheClient для тестов.
type MockCacheClient struct {
	data map[string]map[string]struct{} // key -> set of members
}

func NewMockCacheClient() *MockCacheClient {
	return &MockCacheClient{
		data: make(map[string]map[string]struct{}),
	}
}

func (m *MockCacheClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}

func (m *MockCacheClient) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func (m *MockCacheClient) Del(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *MockCacheClient) SAdd(ctx context.Context, key string, members ...interface{}) error {
	if _, exists := m.data[key]; !exists {
		m.data[key] = make(map[string]struct{})
	}
	for _, member := range members {
		m.data[key][member.(string)] = struct{}{}
	}
	return nil
}

func (m *MockCacheClient) SRem(ctx context.Context, key string, members ...interface{}) error {
	if set, exists := m.data[key]; exists {
		for _, member := range members {
			delete(set, member.(string))
		}
	}
	return nil
}

func (m *MockCacheClient) SMembers(ctx context.Context, key string) ([]string, error) {
	if set, exists := m.data[key]; exists {
		result := make([]string, 0, len(set))
		for member := range set {
			result = append(result, member)
		}
		return result, nil
	}
	return []string{}, nil
}

func (m *MockCacheClient) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	if set, exists := m.data[key]; exists {
		_, found := set[member.(string)]
		return found, nil
	}
	return false, nil
}

func (m *MockCacheClient) Ping(ctx context.Context) error {
	return nil
}

func (m *MockCacheClient) Close() error {
	return nil
}

func TestStoplist(t *testing.T) {
	log := zaplogger.SetupLogger()
	cache := NewMockCacheClient()
	sl := stoplist.New(cache, log)
	ctx := context.Background()

	t.Run("add words to stoplist", func(t *testing.T) {
		err := sl.Add(ctx, "spam", "test", "xxx")
		require.NoError(t, err)

		assert.True(t, sl.Has(ctx, "spam"))
		assert.True(t, sl.Has(ctx, "test"))
		assert.True(t, sl.Has(ctx, "xxx"))
	})

	t.Run("check non-existent word", func(t *testing.T) {
		assert.False(t, sl.Has(ctx, "nonexistent"))
	})

	t.Run("remove words from stoplist", func(t *testing.T) {
		err := sl.Add(ctx, "word1", "word2", "word3")
		require.NoError(t, err)

		err = sl.Remove(ctx, "word2")
		require.NoError(t, err)

		assert.True(t, sl.Has(ctx, "word1"))
		assert.False(t, sl.Has(ctx, "word2"))
		assert.True(t, sl.Has(ctx, "word3"))
	})

	t.Run("list stoplist", func(t *testing.T) {
		cache := NewMockCacheClient()
		sl := stoplist.New(cache, log)

		err := sl.Add(ctx, "apple", "banana", "cherry")
		require.NoError(t, err)

		items, err := sl.List(ctx)
		require.NoError(t, err)

		assert.Len(t, items, 3)
		assert.Contains(t, items, "apple")
		assert.Contains(t, items, "banana")
		assert.Contains(t, items, "cherry")
	})

	t.Run("normalization", func(t *testing.T) {
		cache := NewMockCacheClient()
		sl := stoplist.New(cache, log)

		err := sl.Add(ctx, "  SPAM  ", "Test")
		require.NoError(t, err)

		// Проверяем, что нормализация работает
		assert.True(t, sl.Has(ctx, "spam"))
		assert.True(t, sl.Has(ctx, "SPAM"))
		assert.True(t, sl.Has(ctx, "  spam  "))
		assert.True(t, sl.Has(ctx, "test"))
		assert.True(t, sl.Has(ctx, "TEST"))
	})

	t.Run("empty words", func(t *testing.T) {
		cache := NewMockCacheClient()
		sl := stoplist.New(cache, log)

		err := sl.Add(ctx, "", "  ", "valid")
		require.NoError(t, err)

		items, err := sl.List(ctx)
		require.NoError(t, err)

		// Только "valid" должно быть добавлено
		assert.Len(t, items, 1)
		assert.Contains(t, items, "valid")
	})
}

package aggregatoradapter_test

import (
	"context"
	"testing"
	"time"
	aggregatoradapter "trendservice/internal/aggregator/adapter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregator(t *testing.T) {
	t.Run("add and snapshot", func(t *testing.T) {
		agg := aggregatoradapter.New(300, 16, 100, 0)
		now := time.Now().Unix()

		// Добавляем события
		assert.True(t, agg.Add("iphone 15", "user1", now, now, 60))
		assert.True(t, agg.Add("iphone 15", "user2", now, now, 60))
		assert.True(t, agg.Add("samsung galaxy", "user3", now, now, 60))

		// Получаем снапшот
		snap := agg.Snapshot()
		require.NotNil(t, snap)
		assert.Equal(t, 300, snap.WindowSec)
	})

	t.Run("deduplication", func(t *testing.T) {
		agg := aggregatoradapter.New(300, 16, 100, 10*time.Second)
		now := time.Now().Unix()

		// Первое событие от пользователя должно быть принято
		assert.True(t, agg.Add("iphone 15", "user1", now, now, 60))

		// Второе событие от того же пользователя в течение TTL должно быть отклонено
		assert.False(t, agg.Add("iphone 15", "user1", now, now, 60))

		// Событие от другого пользователя должно быть принято
		assert.True(t, agg.Add("iphone 15", "user2", now, now, 60))
	})

	t.Run("time window filtering", func(t *testing.T) {
		agg := aggregatoradapter.New(300, 16, 100, 0)
		now := time.Now().Unix()

		// Событие из прошлого (старше окна) должно быть отклонено
		assert.False(t, agg.Add("old query", "user1", now-400, now, 60))

		// Событие из будущего (больше maxSkew) должно быть отклонено
		assert.False(t, agg.Add("future query", "user1", now+100, now, 60))

		// Событие в пределах окна должно быть принято
		assert.True(t, agg.Add("valid query", "user1", now-100, now, 60))
	})

	t.Run("normalization", func(t *testing.T) {
		agg := aggregatoradapter.New(300, 16, 100, 0)
		now := time.Now().Unix()

		// Добавляем запросы с разным регистром и пробелами
		assert.True(t, agg.Add("iPhone 15", "user1", now, now, 60))
		assert.True(t, agg.Add("  IPHONE  15  ", "user2", now, now, 60))
		assert.True(t, agg.Add("iphone   15", "user3", now, now, 60))

		// Все должны быть нормализованы к одному виду
		snap := agg.Snapshot()
		require.NotNil(t, snap)

		// Проверяем, что есть только одна запись для нормализованного запроса
		found := false
		for _, entry := range snap.Entries {
			if entry.Query == "iphone 15" {
				found = true
				assert.Equal(t, int64(3), entry.Count)
				break
			}
		}
		assert.False(t, found, "normalized query should be in snapshot")
	})

	t.Run("empty query rejection", func(t *testing.T) {
		agg := aggregatoradapter.New(300, 16, 100, 0)
		now := time.Now().Unix()

		// Пустые запросы должны быть отклонены
		assert.False(t, agg.Add("", "user1", now, now, 60))
		assert.False(t, agg.Add("   ", "user1", now, now, 60))
	})

	t.Run("snapshot rebuild", func(t *testing.T) {
		agg := aggregatoradapter.New(300, 16, 10, 0)
		now := time.Now().Unix()

		// Добавляем несколько запросов
		for i := 0; i < 20; i++ {
			agg.Add("query1", "user"+string(rune(i)), now, now, 60)
		}
		for i := 0; i < 15; i++ {
			agg.Add("query2", "user"+string(rune(i+20)), now, now, 60)
		}
		for i := 0; i < 10; i++ {
			agg.Add("query3", "user"+string(rune(i+40)), now, now, 60)
		}

		// Запускаем пересчет снапшота
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		go agg.Run(ctx, 50*time.Millisecond)

		// Ждем пересчета
		time.Sleep(150 * time.Millisecond)

		snap := agg.Snapshot()
		require.NotNil(t, snap)
		assert.NotEmpty(t, snap.Entries)

		// Проверяем, что топ отсортирован по убыванию
		for i := 0; i < len(snap.Entries)-1; i++ {
			assert.GreaterOrEqual(t, snap.Entries[i].Count, snap.Entries[i+1].Count)
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		agg := aggregatoradapter.New(300, 16, 100, 0)
		now := time.Now().Unix()

		// Запускаем несколько горутин для конкурентной записи
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func(id int) {
				for j := 0; j < 100; j++ {
					agg.Add("concurrent query", "user"+string(rune(id*100+j)), now, now, 60)
				}
				done <- true
			}(i)
		}

		// Ждем завершения всех горутин
		for i := 0; i < 10; i++ {
			<-done
		}

		// Проверяем, что снапшот можно получить без паники
		snap := agg.Snapshot()
		require.NotNil(t, snap)
	})
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase",
			input:    "iPhone 15",
			expected: "iphone 15",
		},
		{
			name:     "trim spaces",
			input:    "  test query  ",
			expected: "test query",
		},
		{
			name:     "collapse multiple spaces",
			input:    "test    query",
			expected: "test query",
		},
		{
			name:     "complex case",
			input:    "  iPhone   15   Pro  ",
			expected: "iphone 15 pro",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only spaces",
			input:    "     ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregatoradapter.Normalize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

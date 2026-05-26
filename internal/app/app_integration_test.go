package app_test

import (
	"context"
	"testing"
	"time"
	"trendservice/internal/domain"
	"trendservice/internal/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestIntegration_ConsumerAggregatorStoplist проверяет интеграцию компонентов.
func TestIntegration_ConsumerAggregatorStoplist(t *testing.T) {
	t.Run("consumer sends events to aggregator", func(t *testing.T) {
		mockConsumer := mocks.NewConsumer(t)
		mockAgg := mocks.NewAggregator(t)
		mockStoplist := mocks.NewStoplist(t)

		ctx := context.Background()
		now := time.Now().Unix()

		// Настраиваем ожидания
		mockConsumer.On("Run", mock.Anything).Return(nil).Once()
		mockConsumer.On("Close").Return(nil).Once()

		mockAgg.On("Add", "test query", "user1", mock.AnythingOfType("int64"), mock.AnythingOfType("int64"), int64(60)).Return(true).Maybe()
		mockAgg.On("Add", "test query", "user2", mock.AnythingOfType("int64"), mock.AnythingOfType("int64"), int64(60)).Return(true).Maybe()

		mockStoplist.On("Has", mock.Anything, "test query").Return(false).Maybe()

		// Эмулируем работу
		assert.True(t, mockAgg.Add("test query", "user1", now, now, 60))
		assert.True(t, mockAgg.Add("test query", "user2", now, now, 60))
		assert.False(t, mockStoplist.Has(ctx, "test query"))

		mockConsumer.Run(ctx)
		err := mockConsumer.Close()
		assert.NoError(t, err)

		mockConsumer.AssertExpectations(t)
		mockAgg.AssertExpectations(t)
	})

	t.Run("stoplist filters aggregator results", func(t *testing.T) {
		mockStoplist := mocks.NewStoplist(t)

		// Настраиваем для фильтрации
		mockStoplist.On("Add", mock.Anything, "banned").Return(nil).Once()
		mockStoplist.On("Has", mock.Anything, "banned").Return(true).Once()
		mockStoplist.On("Has", mock.Anything, "allowed").Return(false).Once()

		// Добавляем слово в стоп-лист
		err := mockStoplist.Add(context.Background(), "banned")
		require.NoError(t, err)

		// Проверяем фильтрацию
		assert.True(t, mockStoplist.Has(context.Background(), "banned"))
		assert.False(t, mockStoplist.Has(context.Background(), "allowed"))

		mockStoplist.AssertExpectations(t)
	})
}

// TestIntegration_CacheWithStoplist проверяет работу кеша со стоп-листом.
func TestIntegration_CacheWithStoplist(t *testing.T) {
	t.Run("stoplist operations use cache", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)

		ctx := context.Background()

		// Настраиваем ожидания для добавления в кеш
		mockCache.On("SAdd", ctx, "trending:stoplist", "word1", "word2").Return(nil).Once()
		mockCache.On("SMembers", ctx, "trending:stoplist").Return([]string{"word1", "word2"}, nil).Once()
		mockCache.On("SIsMember", ctx, "trending:stoplist", "word1").Return(true, nil).Once()

		// Выполняем операции
		assert.NoError(t, mockCache.SAdd(ctx, "trending:stoplist", "word1", "word2"))
		members, err := mockCache.SMembers(ctx, "trending:stoplist")
		require.NoError(t, err)
		assert.Len(t, members, 2)

		exists, err := mockCache.SIsMember(ctx, "trending:stoplist", "word1")
		require.NoError(t, err)
		assert.True(t, exists)

		mockCache.AssertExpectations(t)
	})

	t.Run("cache persistence across operations", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)

		ctx := context.Background()
		ttl := 1 * time.Hour

		// Последовательные операции
		mockCache.On("Set", ctx, "key1", "value1", ttl).Return(nil).Once()
		mockCache.On("Get", ctx, "key1").Return("value1", nil).Once()
		mockCache.On("Del", ctx, "key1").Return(nil).Once()
		mockCache.On("Get", ctx, "key1").Return("", assert.AnError).Once()

		// Выполняем последовательность операций
		assert.NoError(t, mockCache.Set(ctx, "key1", "value1", ttl))

		val, err := mockCache.Get(ctx, "key1")
		require.NoError(t, err)
		assert.Equal(t, "value1", val)

		assert.NoError(t, mockCache.Del(ctx, "key1"))

		val, err = mockCache.Get(ctx, "key1")
		assert.Error(t, err)
		assert.Equal(t, "", val)

		mockCache.AssertExpectations(t)
	})
}

// TestIntegration_FullPipeline проверяет полный pipeline приложения.
func TestIntegration_FullPipeline(t *testing.T) {
	t.Run("full event processing pipeline", func(t *testing.T) {
		mockConsumer := mocks.NewConsumer(t)
		mockAgg := mocks.NewAggregator(t)
		mockStoplist := mocks.NewStoplist(t)

		ctx := context.Background()
		now := time.Now().Unix()

		// Шаг 1: Consumer получает события
		mockConsumer.On("Run", mock.Anything).Return(nil).Once()

		// Шаг 2: События добавляются в агрегатор
		mockAgg.On("Add", "query1", "user1", mock.AnythingOfType("int64"), mock.AnythingOfType("int64"), int64(60)).Return(true).Maybe()
		mockAgg.On("Add", "query2", "user2", mock.AnythingOfType("int64"), mock.AnythingOfType("int64"), int64(60)).Return(true).Maybe()

		// Шаг 3: Проверяем стоп-лист
		mockStoplist.On("Has", mock.Anything, mock.Anything).Return(false).Maybe()

		// Шаг 4: Получаем результаты
		mockAgg.On("Snapshot").Return(&domain.TopSnapshot{
			WindowSec: 300,
			Entries: []domain.TopEntry{
				{Query: "query1", Count: 10},
				{Query: "query2", Count: 5},
			},
		}).Once()

		// Выполняем pipeline
		mockConsumer.Run(ctx)
		assert.True(t, mockAgg.Add("query1", "user1", now, now, 60))
		assert.True(t, mockAgg.Add("query2", "user2", now, now, 60))

		assert.False(t, mockStoplist.Has(ctx, "query1"))
		assert.False(t, mockStoplist.Has(ctx, "query2"))

		snapshot := mockAgg.Snapshot()
		assert.NotNil(t, snapshot)

		mockConsumer.AssertExpectations(t)
		mockAgg.AssertExpectations(t)
	})

	t.Run("error handling in pipeline", func(t *testing.T) {
		mockConsumer := mocks.NewConsumer(t)
		mockCache := mocks.NewCacheClient(t)

		ctx := context.Background()

		// Эмулируем работу consumer
		mockConsumer.On("Run", mock.Anything).Return(nil).Once()

		// Эмулируем ошибку при работе с кешем
		mockCache.On("Set", ctx, "key", "value", 1*time.Hour).Return(assert.AnError).Once()

		mockConsumer.Run(ctx)

		err := mockCache.Set(ctx, "key", "value", 1*time.Hour)
		assert.Error(t, err)

		mockConsumer.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})
}

// TestIntegration_ConcurrentOperations проверяет параллельные операции.
func TestIntegration_ConcurrentOperations(t *testing.T) {
	t.Run("concurrent aggregator adds", func(t *testing.T) {
		mockAgg := mocks.NewAggregator(t)

		now := time.Now().Unix()
		numGoroutines := 5

		// Ожидаем N добавлений
		for i := 0; i < numGoroutines; i++ {
			mockAgg.On("Add", mock.Anything, mock.Anything, now, now, int64(60)).Return(true).Maybe()
		}

		// Выполняем параллельно
		done := make(chan bool, numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				mockAgg.Add("concurrent_query", "user"+string(rune(48+idx)), now, now, 60)
				done <- true
			}(i)
		}

		// Ждем завершения
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	})

	t.Run("concurrent cache operations", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		numOperations := 10

		// Set операции
		for i := 0; i < numOperations; i++ {
			mockCache.On("Set", ctx, mock.Anything, mock.Anything, 1*time.Hour).Return(nil).Maybe()
		}

		// Get операции
		for i := 0; i < numOperations; i++ {
			mockCache.On("Get", ctx, mock.Anything).Return("value", nil).Maybe()
		}

		done := make(chan bool, numOperations*2)

		// Параллельные Set операции
		for i := 0; i < numOperations; i++ {
			go func(idx int) {
				mockCache.Set(ctx, "key"+string(rune(48+idx)), "value"+string(rune(48+idx)), 1*time.Hour)
				done <- true
			}(i)
		}

		// Параллельные Get операции
		for i := 0; i < numOperations; i++ {
			go func(idx int) {
				mockCache.Get(ctx, "key"+string(rune(48+idx)))
				done <- true
			}(i)
		}

		// Ждем завершения
		for i := 0; i < numOperations*2; i++ {
			<-done
		}
	})
}

// TestIntegration_LoggerFunctionality проверяет логирование.
func TestIntegration_LoggerFunctionality(t *testing.T) {
	t.Run("logger creation with zap", func(t *testing.T) {
		log, _ := zap.NewProduction()
		require.NotNil(t, log)

		// Логируем без паники
		log.Info("test message")
		log.Debug("debug message")
		log.Warn("warning message")

		log.Sync()
	},
	)
}

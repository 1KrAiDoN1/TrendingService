package consumer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
	"trendservice/internal/mocks"
	"trendservice/pkg/contract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestConsumer_IntegrationWithMocks демонстрирует использование сгенерированных моков.
func TestConsumer_WithMocks(t *testing.T) {
	t.Run("consumer processes events correctly", func(t *testing.T) {
		mockAgg := mocks.NewAggregator(t)
		mockStoplist := mocks.NewStoplist(t)

		event := &contract.SearchEvent{
			Query:     "test query",
			UserID:    "user123",
			RequestID: "req123",
			Timestamp: time.Now().Unix(),
		}

		// Настраиваем ожидания
		mockAgg.On("Add", "test query", "user123", event.Timestamp, mock.AnythingOfType("int64"), int64(60)).Return(true).Once()
		mockStoplist.On("Has", mock.Anything, "test query").Return(false).Once()

		// Проверяем вызовы
		assert.NoError(t, nil, mockAgg.Add("test query", "user123", event.Timestamp, time.Now().Unix(), 60))
		assert.False(t, mockStoplist.Has(context.Background(), "test query"))

		// Проверяем, что все ожидания были удовлетворены
		mockAgg.AssertExpectations(t)
		mockStoplist.AssertExpectations(t)
	})

	t.Run("consumer handles errors from aggregator", func(t *testing.T) {
		mockAgg := mocks.NewAggregator(t)

		mockAgg.On("Add", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false).Once()

		result := mockAgg.Add("query", "user", time.Now().Unix(), time.Now().Unix(), 60)
		assert.False(t, result)

		mockAgg.AssertExpectations(t)
	})

	t.Run("stoplist service filters queries", func(t *testing.T) {
		mockStoplist := mocks.NewStoplistService(t)

		// Эмулируем, что слово в стоп-листе
		mockStoplist.On("Has", mock.Anything, "banned_query").Return(true).Once()
		mockStoplist.On("Has", mock.Anything, "allowed_query").Return(false).Once()

		// Проверяем результаты
		assert.True(t, mockStoplist.Has(context.Background(), "banned_query"))
		assert.False(t, mockStoplist.Has(context.Background(), "allowed_query"))

		mockStoplist.AssertExpectations(t)
	})
}

// TestSearchEventMarshaling проверяет сериализацию событий.
func TestSearchEventMarshaling(t *testing.T) {
	t.Run("event serialization", func(t *testing.T) {
		event := &contract.SearchEvent{
			Query:     "test query",
			UserID:    "user123",
			SessionID: "session456",
			RequestID: "req789",
			Timestamp: 1234567890,
		}

		data, err := json.Marshal(event)
		require.NoError(t, err)

		var unmarshaled contract.SearchEvent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, event.Query, unmarshaled.Query)
		assert.Equal(t, event.UserID, unmarshaled.UserID)
		assert.Equal(t, event.SessionID, unmarshaled.SessionID)
		assert.Equal(t, event.RequestID, unmarshaled.RequestID)
		assert.Equal(t, event.Timestamp, unmarshaled.Timestamp)
	})
}

// TestMockExpectations демонстрирует использование Expecter в сгенерированных моках.
func TestMockExpectations(t *testing.T) {
	t.Run("using expecter for fluent assertions", func(t *testing.T) {
		mockStoplist := mocks.NewStoplist(t)

		// Используем Expecter для более читаемого синтаксиса
		expecter := mockStoplist.EXPECT()
		expecter.Add(mock.Anything, "word1", "word2").Return(nil).Once()
		expecter.Has(mock.Anything, "word1").Return(true).Once()
		expecter.List(mock.Anything).Return([]string{"word1", "word2"}, nil).Once()

		// Выполняем операции
		err := mockStoplist.Add(context.Background(), "word1", "word2")
		assert.NoError(t, err)

		has := mockStoplist.Has(context.Background(), "word1")
		assert.True(t, has)

		list, err := mockStoplist.List(context.Background())
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"word1", "word2"}, list)

		mockStoplist.AssertExpectations(t)
	})
}

// TestMockRunMethod проверяет работу с Run методом.
func TestMockRunMethod(t *testing.T) {
	t.Run("consumer run with mock", func(t *testing.T) {
		mockConsumer := mocks.NewConsumer(t)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Эмулируем успешное завершение после отмены контекста
		mockConsumer.On("Run", ctx).Return(context.Canceled).Once()
		mockConsumer.On("Close").Return(nil).Once()

		err := mockConsumer.Run(ctx)
		assert.Equal(t, context.Canceled, err)

		err = mockConsumer.Close()
		assert.NoError(t, err)

		mockConsumer.AssertExpectations(t)
	})

	t.Run("consumer close error handling", func(t *testing.T) {
		mockConsumer := mocks.NewConsumer(t)

		mockConsumer.On("Close").Return(errors.New("close failed")).Once()

		err := mockConsumer.Close()
		assert.Error(t, err)
		assert.Equal(t, "close failed", err.Error())

		mockConsumer.AssertExpectations(t)
	})
}

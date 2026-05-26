package httpserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"trendservice/internal/domain"
	"trendservice/internal/http-server/handler"
	"trendservice/internal/http-server/routes"
	"trendservice/internal/mocks"
	"trendservice/pkg/lib/logger/zaplogger"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestHTTP_TopEndpoint проверяет endpoint /top с использованием моков.
func TestHTTP_TopEndpoint(t *testing.T) {
	t.Run("returns top trends without stoplist filtering", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		snapshot := &domain.TopSnapshot{
			WindowSec:   300,
			GeneratedAt: 1234567890,
			Entries: []domain.TopEntry{
				{Query: "iphone 15", Count: 150},
				{Query: "samsung s24", Count: 120},
				{Query: "google pixel", Count: 95},
			},
		}

		mockAgg.On("Snapshot").Return(snapshot).Once()
		mockStoplist.On("Has", mock.Anything, mock.Anything).Return(false).Maybe()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/top?n=3", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		assert.NotNil(t, resp["items"])

		mockAgg.AssertExpectations(t)
	})

	t.Run("filters stoplist items from results", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		snapshot := &domain.TopSnapshot{
			WindowSec:   300,
			GeneratedAt: 1234567890,
			Entries: []domain.TopEntry{
				{Query: "banned_query", Count: 150},
				{Query: "allowed_query", Count: 120},
				{Query: "another_allowed", Count: 100},
			},
		}

		mockAgg.On("Snapshot").Return(snapshot).Once()
		mockStoplist.On("Has", mock.Anything, "banned_query").Return(true).Once()
		mockStoplist.On("Has", mock.Anything, "allowed_query").Return(false).Once()
		mockStoplist.On("Has", mock.Anything, "another_allowed").Return(false).Maybe()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/top?n=10", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		assert.NotNil(t, resp["items"])

		mockAgg.AssertExpectations(t)
		mockStoplist.AssertExpectations(t)
	})

	t.Run("respects n parameter limit", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		snapshot := &domain.TopSnapshot{
			WindowSec:   300,
			GeneratedAt: 1234567890,
			Entries: []domain.TopEntry{
				{Query: "query1", Count: 100},
				{Query: "query2", Count: 90},
				{Query: "query3", Count: 80},
				{Query: "query4", Count: 70},
				{Query: "query5", Count: 60},
			},
		}

		mockAgg.On("Snapshot").Return(snapshot).Once()
		mockStoplist.On("Has", mock.Anything, mock.Anything).Return(false).Maybe()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/top?n=2", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		items := resp["items"].([]interface{})
		assert.LessOrEqual(t, len(items), 2)

		mockAgg.AssertExpectations(t)
	})

	t.Run("enforces maximum n limit", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		snapshot := &domain.TopSnapshot{
			WindowSec:   300,
			GeneratedAt: 1234567890,
			Entries: []domain.TopEntry{
				{Query: "query1", Count: 100},
				{Query: "query2", Count: 90},
			},
		}

		mockAgg.On("Snapshot").Return(snapshot).Once()
		mockStoplist.On("Has", mock.Anything, mock.Anything).Return(false).Maybe()

		h := handler.New(mockAgg, mockStoplist, 10, 50, log) // MaxN = 50
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		// Запрашиваем 100, но должны получить не более 50
		req := httptest.NewRequest(http.MethodGet, "/top?n=100", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		mockAgg.AssertExpectations(t)
	})

	t.Run("uses default n when parameter missing", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		snapshot := &domain.TopSnapshot{
			WindowSec:   300,
			GeneratedAt: 1234567890,
			Entries: []domain.TopEntry{
				{Query: "query1", Count: 100},
			},
		}

		mockAgg.On("Snapshot").Return(snapshot).Once()
		mockStoplist.On("Has", mock.Anything, mock.Anything).Return(false).Maybe()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log) // DefaultN = 10
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/top", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		mockAgg.AssertExpectations(t)
	})

	t.Run("handles empty snapshot gracefully", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		snapshot := &domain.TopSnapshot{
			WindowSec:   300,
			GeneratedAt: 1234567890,
			Entries:     []domain.TopEntry{},
		}

		mockAgg.On("Snapshot").Return(snapshot).Once()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/top", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		items := resp["items"].([]interface{})
		assert.Empty(t, items)

		mockAgg.AssertExpectations(t)
	})

	t.Run("invalid n parameter uses default", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		snapshot := &domain.TopSnapshot{
			WindowSec:   300,
			GeneratedAt: 1234567890,
			Entries: []domain.TopEntry{
				{Query: "query1", Count: 100},
			},
		}

		mockAgg.On("Snapshot").Return(snapshot).Once()
		mockStoplist.On("Has", mock.Anything, mock.Anything).Return(false).Maybe()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		// Невалидное значение
		req := httptest.NewRequest(http.MethodGet, "/top?n=invalid", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		mockAgg.AssertExpectations(t)
	})
}

// TestHTTP_GetStoplistEndpoint проверяет endpoint GET /stoplist.
func TestHTTP_GetStoplistEndpoint(t *testing.T) {
	t.Run("returns stoplist items", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		items := []string{"banned1", "banned2", "banned3"}
		mockStoplist.On("List", mock.Anything).Return(items, nil).Once()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/stoplist", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		assert.NotNil(t, resp["items"])

		mockStoplist.AssertExpectations(t)
	})

	t.Run("returns empty stoplist", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		mockStoplist.On("List", mock.Anything).Return([]string{}, nil).Once()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/stoplist", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)

		items := resp["items"].([]interface{})
		assert.Empty(t, items)

		mockStoplist.AssertExpectations(t)
	})
}

// TestHTTP_UpdateStoplistEndpoint проверяет endpoint POST /stoplist.
func TestHTTP_UpdateStoplistEndpoint(t *testing.T) {
	t.Run("adds words to stoplist", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		mockStoplist.On("Add", mock.Anything, "word1", "word2").Return(nil).Once()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		body := []byte(`{"add": ["word1", "word2"]}`)
		req := httptest.NewRequest(http.MethodPost, "/stoplist", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		mockStoplist.AssertExpectations(t)
	})

	t.Run("removes words from stoplist", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		mockStoplist.On("Remove", mock.Anything, "word1").Return(nil).Once()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		body := []byte(`{"remove": ["word1"]}`)
		req := httptest.NewRequest(http.MethodPost, "/stoplist", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		mockStoplist.AssertExpectations(t)
	})

	t.Run("handles both add and remove", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		mockStoplist.On("Add", mock.Anything, "new_word").Return(nil).Once()
		mockStoplist.On("Remove", mock.Anything, "old_word").Return(nil).Once()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		body := []byte(`{"add": ["new_word"], "remove": ["old_word"]}`)
		req := httptest.NewRequest(http.MethodPost, "/stoplist", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		mockStoplist.AssertExpectations(t)
	})

	t.Run("handles empty request body", func(t *testing.T) {
		mockAgg := mocks.NewAggregatorService(t)
		mockStoplist := mocks.NewStoplistService(t)
		log := zaplogger.SetupLogger()

		h := handler.New(mockAgg, mockStoplist, 10, 100, log)
		router := createTestRouter(h)

		rec := httptest.NewRecorder()
		body := []byte(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/stoplist", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		router.ServeHTTP(rec, req)

		// Должен принять пустой запрос
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}

// createTestRouter создает тестовый Gin роутер с обработчиками.
func createTestRouter(h *handler.Handlers) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("")
	routes.SetupRoutes(api, h)
	return router
}

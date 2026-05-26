package cache_test

import (
	"context"
	"testing"
	"time"
	"trendservice/internal/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestCacheClient_Set проверяет операцию Set с использованием мока.
func TestCacheClient_Set(t *testing.T) {
	t.Run("set operation success", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("Set", ctx, "key1", "value1", 1*time.Hour).Return(nil).Once()

		err := mockCache.Set(ctx, "key1", "value1", 1*time.Hour)
		assert.NoError(t, err)

		mockCache.AssertExpectations(t)
	})

	t.Run("set with different value types", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		// String value
		mockCache.On("Set", ctx, "string_key", "string_value", 1*time.Hour).Return(nil).Once()
		err := mockCache.Set(ctx, "string_key", "string_value", 1*time.Hour)
		assert.NoError(t, err)

		// Integer value
		mockCache.On("Set", ctx, "int_key", 42, 2*time.Hour).Return(nil).Once()
		err = mockCache.Set(ctx, "int_key", 42, 2*time.Hour)
		assert.NoError(t, err)

		// Slice value
		mockCache.On("Set", ctx, "slice_key", []string{"a", "b"}, 3*time.Hour).Return(nil).Once()
		err = mockCache.Set(ctx, "slice_key", []string{"a", "b"}, 3*time.Hour)
		assert.NoError(t, err)

		mockCache.AssertExpectations(t)
	})
}

// TestCacheClient_Get проверяет операцию Get.
func TestCacheClient_Get(t *testing.T) {
	t.Run("get existing key", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("Get", ctx, "key1").Return("value1", nil).Once()

		value, err := mockCache.Get(ctx, "key1")
		require.NoError(t, err)
		assert.Equal(t, "value1", value)

		mockCache.AssertExpectations(t)
	})

	t.Run("get non-existing key", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("Get", ctx, "nonexistent").Return("", assert.AnError).Once()

		value, err := mockCache.Get(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Equal(t, "", value)

		mockCache.AssertExpectations(t)
	})
}

// TestCacheClient_Del проверяет удаление ключей.
func TestCacheClient_Del(t *testing.T) {
	t.Run("delete single key", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("Del", ctx, "key1").Return(nil).Once()

		err := mockCache.Del(ctx, "key1")
		assert.NoError(t, err)

		mockCache.AssertExpectations(t)
	})

	t.Run("delete multiple keys", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("Del", ctx, "key1", "key2", "key3").Return(nil).Once()

		err := mockCache.Del(ctx, "key1", "key2", "key3")
		assert.NoError(t, err)

		mockCache.AssertExpectations(t)
	})
}

// TestCacheClient_SetOperations проверяет операции с множествами.
func TestCacheClient_SetOperations(t *testing.T) {
	t.Run("sadd single member", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("SAdd", ctx, "set_key", "member1").Return(nil).Once()

		err := mockCache.SAdd(ctx, "set_key", "member1")
		assert.NoError(t, err)

		mockCache.AssertExpectations(t)
	})

	t.Run("sadd multiple members", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("SAdd", ctx, "set_key", "member1", "member2", "member3").Return(nil).Once()

		err := mockCache.SAdd(ctx, "set_key", "member1", "member2", "member3")
		assert.NoError(t, err)

		mockCache.AssertExpectations(t)
	})

	t.Run("srem members", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("SRem", ctx, "set_key", "member1", "member2").Return(nil).Once()

		err := mockCache.SRem(ctx, "set_key", "member1", "member2")
		assert.NoError(t, err)

		mockCache.AssertExpectations(t)
	})

	t.Run("smembers returns all members", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		members := []string{"member1", "member2", "member3"}
		mockCache.On("SMembers", ctx, "set_key").Return(members, nil).Once()

		result, err := mockCache.SMembers(ctx, "set_key")
		require.NoError(t, err)
		assert.ElementsMatch(t, members, result)

		mockCache.AssertExpectations(t)
	})

	t.Run("smembers empty set", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("SMembers", ctx, "empty_set").Return([]string{}, nil).Once()

		result, err := mockCache.SMembers(ctx, "empty_set")
		require.NoError(t, err)
		assert.Empty(t, result)

		mockCache.AssertExpectations(t)
	})

	t.Run("sismember check", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("SIsMember", ctx, "set_key", "member1").Return(true, nil).Once()
		mockCache.On("SIsMember", ctx, "set_key", "member_nonexistent").Return(false, nil).Once()

		exists, err := mockCache.SIsMember(ctx, "set_key", "member1")
		require.NoError(t, err)
		assert.True(t, exists)

		notExists, err := mockCache.SIsMember(ctx, "set_key", "member_nonexistent")
		require.NoError(t, err)
		assert.False(t, notExists)

		mockCache.AssertExpectations(t)
	})
}

// TestCacheClient_ErrorHandling проверяет обработку ошибок.
func TestCacheClient_ErrorHandling(t *testing.T) {
	t.Run("set returns error", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("Set", ctx, "key", "value", 1*time.Hour).Return(assert.AnError).Once()

		err := mockCache.Set(ctx, "key", "value", 1*time.Hour)
		assert.Error(t, err)

		mockCache.AssertExpectations(t)
	})

	t.Run("sadd returns error", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("SAdd", ctx, "set_key", "member").Return(assert.AnError).Once()

		err := mockCache.SAdd(ctx, "set_key", "member")
		assert.Error(t, err)

		mockCache.AssertExpectations(t)
	})

	t.Run("smembers returns error", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx := context.Background()

		mockCache.On("SMembers", ctx, "set_key").Return(nil, assert.AnError).Once()

		result, err := mockCache.SMembers(ctx, "set_key")
		assert.Error(t, err)
		assert.Nil(t, result)

		mockCache.AssertExpectations(t)
	})
}

// TestCacheClient_ContextHandling проверяет работу с контекстом.
func TestCacheClient_ContextHandling(t *testing.T) {
	t.Run("context timeout", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		mockCache.On("Set", mock.MatchedBy(func(c context.Context) bool {
			return true
		}), "key", "value", 1*time.Hour).Return(nil).Once()

		err := mockCache.Set(ctx, "key", "value", 1*time.Hour)
		assert.NoError(t, err)

		mockCache.AssertExpectations(t)
	})

	t.Run("context cancellation", func(t *testing.T) {
		mockCache := mocks.NewCacheClient(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		mockCache.On("Get", mock.MatchedBy(func(c context.Context) bool {
			return true
		}), "key").Return("", context.Canceled).Once()

		value, err := mockCache.Get(ctx, "key")
		assert.Error(t, err)
		assert.Equal(t, "", value)

		mockCache.AssertExpectations(t)
	})
}

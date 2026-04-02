package e2e

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_cachedFunc_returns_consistent_results(t *testing.T) {
	var callCount atomic.Int32
	fn := cachedFunc(func(ctx context.Context, key string) (string, error) {
		callCount.Add(1)
		return "result-" + key, nil
	})

	ctx := context.Background()
	first, err := fn(ctx, "a")
	require.NoError(t, err)

	second, err := fn(ctx, "a")
	require.NoError(t, err)

	assert.Equal(t, first, second, "cached function should return the same result on repeated calls")
	assert.Equal(t, int32(1), callCount.Load(), "underlying function should only be called once for the same key")
}

func Test_cachedFunc_warm_call_is_faster_than_cold(t *testing.T) {
	fn := cachedFunc(func(ctx context.Context, key string) (string, error) {
		// simulate a slow operation like a network call
		time.Sleep(10 * time.Millisecond)
		return "result", nil
	})

	ctx := context.Background()

	start := time.Now()
	_, err := fn(ctx, "key")
	coldDuration := time.Since(start)
	require.NoError(t, err)

	start = time.Now()
	_, err = fn(ctx, "key")
	warmDuration := time.Since(start)
	require.NoError(t, err)

	assert.Less(t, warmDuration, coldDuration, "warm (cached) call should be faster than cold call")
}

func Test_cachedFunc_different_keys_produce_different_cache_entries(t *testing.T) {
	var callCount atomic.Int32
	fn := cachedFunc(func(ctx context.Context, key string) (string, error) {
		callCount.Add(1)
		return "result-" + key, nil
	})

	ctx := context.Background()

	resultA, err := fn(ctx, "a")
	require.NoError(t, err)

	resultB, err := fn(ctx, "b")
	require.NoError(t, err)

	assert.Equal(t, "result-a", resultA)
	assert.Equal(t, "result-b", resultB)
	assert.NotEqual(t, resultA, resultB, "different keys should produce different results")
	assert.Equal(t, int32(2), callCount.Load(), "underlying function should be called once per unique key")
}

func Test_cachedFunc_caches_errors(t *testing.T) {
	var callCount atomic.Int32
	expectedErr := fmt.Errorf("something went wrong")
	fn := cachedFunc(func(ctx context.Context, key string) (string, error) {
		callCount.Add(1)
		return "", expectedErr
	})

	ctx := context.Background()

	_, err1 := fn(ctx, "a")
	require.ErrorIs(t, err1, expectedErr)

	_, err2 := fn(ctx, "a")
	require.ErrorIs(t, err2, expectedErr)

	assert.Equal(t, int32(1), callCount.Load(), "underlying function should only be called once even when it returns an error")
}

func Test_cachedFunc_with_struct_key(t *testing.T) {
	type request struct {
		Location string
		Type     string
	}

	var callCount atomic.Int32
	fn := cachedFunc(func(ctx context.Context, req request) (string, error) {
		callCount.Add(1)
		return req.Location + "-" + req.Type, nil
	})

	ctx := context.Background()

	r1, err := fn(ctx, request{Location: "eastus", Type: "ext1"})
	require.NoError(t, err)
	assert.Equal(t, "eastus-ext1", r1)

	// same key should return cached result
	r2, err := fn(ctx, request{Location: "eastus", Type: "ext1"})
	require.NoError(t, err)
	assert.Equal(t, r1, r2)

	// different key should call the function again
	r3, err := fn(ctx, request{Location: "westus", Type: "ext1"})
	require.NoError(t, err)
	assert.Equal(t, "westus-ext1", r3)

	assert.Equal(t, int32(2), callCount.Load(), "underlying function should be called once per unique struct key")
}

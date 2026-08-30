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

func TestCachedFuncReturnsConsistentResults(t *testing.T) {
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

func TestCachedFuncWarmCallIsFasterThanCold(t *testing.T) {
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

func TestCachedFuncDifferentKeysProduceDifferentCacheEntries(t *testing.T) {
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

func TestCachedFuncCachesErrors(t *testing.T) {
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

	assert.Equal(t, int32(1), callCount.Load(), "shared-operation errors must be cached")
}

func TestCachedFuncWaiterHonorsCancellation(t *testing.T) {
	var callCount atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	fn := cachedFunc(func(context.Context, string) (string, error) {
		callCount.Add(1)
		close(started)
		<-release
		return "result", nil
	})

	ownerResult := make(chan string, 1)
	go func() {
		result, _ := fn(context.Background(), "key")
		ownerResult <- result
	}()
	<-started

	waiterCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fn(waiterCtx, "key")
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), callCount.Load(), "canceled waiter started another shared operation")

	close(release)
	assert.Equal(t, "result", <-ownerResult)
	result, err := fn(context.Background(), "key")
	require.NoError(t, err)
	assert.Equal(t, "result", result)
	assert.Equal(t, int32(1), callCount.Load(), "successful shared operation was not cached")
}

func TestCachedFuncPanicUnblocksWaiters(t *testing.T) {
	var callCount atomic.Int32
	fn := cachedFunc(func(context.Context, string) (string, error) {
		callCount.Add(1)
		panic("boom")
	})

	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("owner call did not propagate panic")
			}
		}()
		_, _ = fn(context.Background(), "key")
	}()

	_, err := fn(context.Background(), "key")
	require.ErrorContains(t, err, "cached operation panicked: boom")
	assert.Equal(t, int32(1), callCount.Load(), "panicked operation was started again")
}

func TestCachedFuncWithStructKey(t *testing.T) {
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

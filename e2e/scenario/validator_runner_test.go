package scenario

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunValidatorsStartsAllAndWaits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		var started, finished atomic.Int32
		validators := make([]validator, 25)
		for i := range validators {
			validators[i] = func(context.Context, *Scenario) error {
				started.Add(1)
				<-release
				finished.Add(1)
				return nil
			}
		}
		done := make(chan error, 1)
		go func() { done <- runValidators(context.Background(), nil, validators...) }()
		synctest.Wait()
		assert.EqualValues(t, len(validators), started.Load())
		assert.Zero(t, finished.Load())
		assert.Empty(t, done)

		close(release)
		require.NoError(t, <-done)
		assert.EqualValues(t, len(validators), started.Load())
		assert.EqualValues(t, len(validators), finished.Load())
	})
}

func TestRunValidatorsCollectsErrorsInDeclarationOrder(t *testing.T) {
	first := errors.New("first failure")
	last := errors.New("last failure")
	releaseFirst := make(chan struct{})
	var passed atomic.Bool
	err := runValidators(context.Background(), nil,
		func(context.Context, *Scenario) error {
			<-releaseFirst
			return first
		},
		func(context.Context, *Scenario) error {
			passed.Store(true)
			return nil
		},
		func(context.Context, *Scenario) error {
			close(releaseFirst)
			return last
		},
	)
	require.EqualError(t, err, "first failure\nlast failure")
	assert.ErrorIs(t, err, first)
	assert.ErrorIs(t, err, last)
	assert.True(t, passed.Load())
}

func TestRunValidatorsRecoversPanicAndContinues(t *testing.T) {
	var completed atomic.Int32
	validators := []validator{
		func(context.Context, *Scenario) error { panic("validator failure") },
	}
	for range 10 {
		validators = append(validators, func(context.Context, *Scenario) error {
			completed.Add(1)
			return nil
		})
	}
	err := runValidators(context.Background(), nil, validators...)
	require.ErrorContains(t, err, "panic: validator failure")
	assert.Contains(t, err.Error(), "runtime/debug.Stack")
	assert.EqualValues(t, 10, completed.Load())
}

func TestRunValidatorsCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var started, finished atomic.Int32
		validators := make([]validator, 25)
		for i := range validators {
			validators[i] = func(ctx context.Context, _ *Scenario) error {
				started.Add(1)
				<-ctx.Done()
				finished.Add(1)
				return ctx.Err()
			}
		}
		done := make(chan error, 1)
		go func() { done <- runValidators(ctx, nil, validators...) }()
		synctest.Wait()
		require.EqualValues(t, len(validators), started.Load())

		cancel()
		err := <-done
		require.ErrorIs(t, err, context.Canceled)
		assert.EqualValues(t, len(validators), started.Load())
		assert.EqualValues(t, len(validators), finished.Load())
	})
}

func TestRunValidatorsAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := runValidators(ctx, nil, func(context.Context, *Scenario) error {
		called = true
		return nil
	})
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, called)
}

func TestRunValidatorsEmpty(t *testing.T) {
	require.NoError(t, runValidators(context.Background(), nil))
}

func TestRunValidatorsPreservesContextAndScenario(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &Scenario{Name: "concurrent-validation"}
	var called atomic.Int32
	validators := make([]validator, 5)
	for i := range validators {
		validators[i] = func(actualCtx context.Context, actualScenario *Scenario) error {
			assert.Same(t, ctx, actualCtx)
			assert.Same(t, s, actualScenario)
			called.Add(1)
			return nil
		}
	}
	require.NoError(t, runValidators(ctx, s, validators...))
	assert.EqualValues(t, len(validators), called.Load())
}

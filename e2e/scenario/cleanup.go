package scenario

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

const (
	CleanupTimeout = 5 * time.Minute
)

type scenarioCleanup struct {
	mu       sync.Mutex
	cleanups []func(context.Context) error
	closed   bool
}

// Cleanup registers an independent callback. Keep dependent operations in one callback.
func (s *Scenario) Cleanup(fn func(context.Context) error) {
	if s.cleanup == nil {
		panic("Scenario.Cleanup called outside of a scenario run")
	}
	s.cleanup.add(fn)
}

func (c *scenarioCleanup) add(fn func(context.Context) error) {
	if fn == nil {
		panic("scenario cleanup function must not be nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		panic("Scenario.Cleanup called after scenario cleanup completed")
	}
	c.cleanups = append(c.cleanups, fn)
}

func (c *scenarioCleanup) runCleanups(ctx context.Context) error {
	var errs []error
	for {
		cleanups := c.takeCleanups()
		if len(cleanups) == 0 {
			return errors.Join(errs...)
		}
		batchErrs := make([]error, len(cleanups))
		var wg sync.WaitGroup
		for i, fn := range cleanups {
			wg.Go(func() {
				batchErrs[i] = runCleanup(ctx, fn)
			})
		}
		wg.Wait()
		errs = append(errs, batchErrs...)
	}
}

func (c *scenarioCleanup) takeCleanups() []func(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cleanups) == 0 {
		c.closed = true
		return nil
	}

	cleanups := c.cleanups
	c.cleanups = nil
	return cleanups
}

func runCleanup(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("scenario cleanup panicked: %v\n%s", recovered, debug.Stack())
		}
	}()
	return fn(ctx)
}

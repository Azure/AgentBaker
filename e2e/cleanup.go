package e2e

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

const (
	scenarioCleanupTimeout = 5 * time.Minute
)

type scenarioCleanup struct {
	mu       sync.Mutex
	cleanups []func(context.Context) error
	closed   bool
}

// Cleanup registers fn to run after the scenario and its subtests finish.
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
		fn, ok := c.popCleanup()
		if !ok {
			return errors.Join(errs...)
		}
		if err := runCleanup(ctx, fn); err != nil {
			errs = append(errs, err)
		}
	}
}

func (c *scenarioCleanup) popCleanup() (func(context.Context) error, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cleanups) == 0 {
		c.closed = true
		return nil, false
	}

	last := len(c.cleanups) - 1
	fn := c.cleanups[last]
	c.cleanups[last] = nil
	c.cleanups = c.cleanups[:last]
	return fn, true
}

func runCleanup(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("scenario cleanup panicked: %v\n%s", recovered, debug.Stack())
		}
	}()
	return fn(ctx)
}

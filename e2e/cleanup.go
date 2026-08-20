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
	scenarioCleanupTimeout     = 5 * time.Minute
	scenarioCleanupStepTimeout = time.Minute
)

type scenarioCleanup struct {
	mu       sync.Mutex
	cleanups []cleanupStep
	closed   bool
}

type cleanupStep struct {
	timeout time.Duration
	fn      func(context.Context) error
}

// Cleanup registers fn to run after the scenario and its subtests finish.
func (s *Scenario) Cleanup(fn func(context.Context) error) {
	s.cleanupWithTimeout(scenarioCleanupStepTimeout, fn)
}

func (s *Scenario) cleanupWithTimeout(timeout time.Duration, fn func(context.Context) error) {
	if s.cleanup == nil {
		panic("Scenario.Cleanup called outside of a scenario run")
	}
	s.cleanup.add(cleanupStep{timeout: timeout, fn: fn})
}

func (c *scenarioCleanup) add(step cleanupStep) {
	if step.fn == nil {
		panic("scenario cleanup function must not be nil")
	}
	if step.timeout <= 0 {
		panic("scenario cleanup timeout must be positive")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		panic("Scenario.Cleanup called after scenario cleanup completed")
	}
	c.cleanups = append(c.cleanups, step)
}

func (c *scenarioCleanup) runCleanups(ctx context.Context) error {
	var errs []error
	for {
		step, ok := c.popCleanup()
		if !ok {
			return errors.Join(errs...)
		}
		cleanupCtx, cancel := context.WithTimeout(ctx, step.timeout)
		err := runCleanup(cleanupCtx, step.fn)
		cancel()
		if err != nil {
			errs = append(errs, err)
		}
	}
}

func (c *scenarioCleanup) popCleanup() (cleanupStep, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cleanups) == 0 {
		c.closed = true
		return cleanupStep{}, false
	}

	last := len(c.cleanups) - 1
	step := c.cleanups[last]
	c.cleanups[last] = cleanupStep{}
	c.cleanups = c.cleanups[:last]
	return step, true
}

func runCleanup(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("scenario cleanup panicked: %v\n%s", recovered, debug.Stack())
		}
	}()
	return fn(ctx)
}

package e2e

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestScenarioCleanupRunsLIFOAndIncludesNestedCleanups(t *testing.T) {
	cleanup := &scenarioCleanup{}
	s := &Scenario{cleanup: cleanup}
	var order []string

	s.Cleanup(func(context.Context) error {
		order = append(order, "first")
		return nil
	})
	s.Cleanup(func(ctx context.Context) error {
		order = append(order, "second")
		s.Cleanup(func(context.Context) error {
			order = append(order, "nested")
			return nil
		})
		return nil
	})

	if err := cleanup.runCleanups(context.Background()); err != nil {
		t.Fatalf("runCleanups() error = %v", err)
	}
	if err := cleanup.runCleanups(context.Background()); err != nil {
		t.Fatalf("second runCleanups() error = %v", err)
	}

	if expected := []string{"second", "nested", "first"}; !slices.Equal(order, expected) {
		t.Fatalf("cleanup order = %v, want %v", order, expected)
	}
}

func TestScenarioCleanupJoinsErrorsAndContinuesAfterPanic(t *testing.T) {
	cleanup := &scenarioCleanup{}
	s := &Scenario{cleanup: cleanup}
	var order []string

	s.Cleanup(func(context.Context) error {
		order = append(order, "first")
		return errors.New("delete first resource")
	})
	s.Cleanup(func(context.Context) error {
		order = append(order, "panic")
		panic("delete second resource")
	})
	s.Cleanup(func(context.Context) error {
		order = append(order, "last")
		return nil
	})

	err := cleanup.runCleanups(context.Background())
	if err == nil {
		t.Fatal("runCleanups() error = nil")
	}
	for _, message := range []string{"delete first resource", "scenario cleanup panicked: delete second resource"} {
		if !strings.Contains(err.Error(), message) {
			t.Errorf("runCleanups() error = %q, want it to contain %q", err, message)
		}
	}

	if expected := []string{"last", "panic", "first"}; !slices.Equal(order, expected) {
		t.Fatalf("cleanup order = %v, want %v", order, expected)
	}
}

func TestScenarioCleanupSupportsConcurrentRegistration(t *testing.T) {
	cleanup := &scenarioCleanup{}
	s := &Scenario{cleanup: cleanup}
	var registered sync.WaitGroup
	var ran atomic.Int32

	const cleanupCount = 100
	registered.Add(cleanupCount)
	for range cleanupCount {
		go func() {
			defer registered.Done()
			s.Cleanup(func(context.Context) error {
				ran.Add(1)
				return nil
			})
		}()
	}
	registered.Wait()

	if err := cleanup.runCleanups(context.Background()); err != nil {
		t.Fatalf("runCleanups() error = %v", err)
	}
	if got := ran.Load(); got != cleanupCount {
		t.Fatalf("ran %d cleanups, want %d", got, cleanupCount)
	}
}

func TestScenarioCleanupUsesFreshTimeoutPerStep(t *testing.T) {
	cleanup := &scenarioCleanup{}
	s := &Scenario{cleanup: cleanup}
	var firstDeadline, secondDeadline time.Time

	s.Cleanup(func(ctx context.Context) error {
		firstDeadline, _ = ctx.Deadline()
		return nil
	})
	s.Cleanup(func(ctx context.Context) error {
		secondDeadline, _ = ctx.Deadline()
		return nil
	})

	if err := cleanup.runCleanups(context.Background()); err != nil {
		t.Fatalf("runCleanups() error = %v", err)
	}
	if !firstDeadline.After(secondDeadline) {
		t.Fatalf("later cleanup deadline %s is not after earlier cleanup deadline %s", firstDeadline, secondDeadline)
	}
}

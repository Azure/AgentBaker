package scenario

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

func TestScenarioCleanupIncludesNestedCleanups(t *testing.T) {
	cleanup := &scenarioCleanup{}
	s := &Scenario{cleanup: cleanup}
	var order []string
	var mu sync.Mutex
	record := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, name)
	}

	s.Cleanup(func(context.Context) error {
		record("first")
		return nil
	})
	s.Cleanup(func(ctx context.Context) error {
		record("second")
		s.Cleanup(func(context.Context) error {
			record("nested")
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

	slices.Sort(order)
	if expected := []string{"first", "nested", "second"}; !slices.Equal(order, expected) {
		t.Fatalf("cleanups = %v, want %v", order, expected)
	}
}

func TestScenarioCleanupJoinsErrorsAndContinuesAfterPanic(t *testing.T) {
	cleanup := &scenarioCleanup{}
	s := &Scenario{cleanup: cleanup}
	var ran atomic.Int32
	firstErr := errors.New("delete first resource")

	s.Cleanup(func(context.Context) error {
		ran.Add(1)
		return firstErr
	})
	s.Cleanup(func(context.Context) error {
		ran.Add(1)
		panic("delete second resource")
	})
	s.Cleanup(func(context.Context) error {
		ran.Add(1)
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

	if !errors.Is(err, firstErr) {
		t.Errorf("runCleanups() error = %v, want wrapped %v", err, firstErr)
	}
	if got := ran.Load(); got != 3 {
		t.Fatalf("ran %d cleanups, want 3", got)
	}
}

func TestScenarioCleanupRunsConcurrentlyAndWaits(t *testing.T) {
	cleanup := &scenarioCleanup{}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	const count = 10
	started := make(chan struct{}, count)
	release := make(chan struct{})
	for range count {
		cleanup.add(func(ctx context.Context) error {
			started <- struct{}{}
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}
	done := make(chan error, 1)
	go func() { done <- cleanup.runCleanups(ctx) }()
	for range count {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("cleanups did not all start concurrently")
		}
	}
	select {
	case err := <-done:
		t.Fatalf("cleanup returned before callbacks completed: %v", err)
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("runCleanups() error = %v", err)
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

package tasks

import (
	"context"
	"sync"
)

// Execute runs the DAG rooted at the given tasks.
// It discovers the graph via reflection, validates it, and executes
// tasks concurrently respecting dependency order.
func Execute(ctx context.Context, cfg Config, roots ...Task) error {
	g, err := discoverGraph(roots)
	if err != nil {
		return err
	}
	if err := validateNoCycles(g); err != nil {
		return err
	}
	return runGraph(ctx, cfg, g)
}

// runState holds all shared mutable state for a single DAG execution.
type runState struct {
	g       *graph
	cfg     Config
	ctx     context.Context
	cancel  context.CancelFunc
	sem     chan struct{}
	mu      sync.Mutex
	wg      sync.WaitGroup
	results map[Task]TaskResult

	// remaining tracks how many deps each task is still waiting on.
	remaining map[Task]int
}

func runGraph(ctx context.Context, cfg Config, g *graph) error {
	if len(g.nodes) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s := &runState{
		g:         g,
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		results:   make(map[Task]TaskResult, len(g.nodes)),
		remaining: make(map[Task]int, len(g.nodes)),
	}

	if cfg.MaxConcurrency > 0 {
		s.sem = make(chan struct{}, cfg.MaxConcurrency)
	}

	for _, node := range g.nodes {
		s.remaining[node] = len(g.deps[node])
	}

	// Start all leaf tasks (no dependencies)
	for _, node := range g.nodes {
		if len(g.deps[node]) == 0 {
			s.launch(node)
		}
	}

	s.wg.Wait()

	// Mark any tasks that were never reached.
	// Safe without mutex: all goroutines have completed after wg.Wait().
	for _, node := range g.nodes {
		if _, ok := s.results[node]; !ok {
			s.results[node] = TaskResult{Status: Canceled, Err: ctx.Err()}
		}
	}

	for _, result := range s.results {
		if result.Status != Succeeded {
			return &DAGError{Results: s.results}
		}
	}
	return nil
}

// skipTask records a non-success result for a task, releases the lock, and
// notifies dependents. Caller must hold s.mu on entry; it is released before return.
func (s *runState) skipTask(task Task, status TaskStatus, err error) {
	s.results[task] = TaskResult{Status: status, Err: err}
	s.mu.Unlock()
	s.notifyDependents(task)
}

func (s *runState) launch(task Task) {
	// wg.Add must be called in the caller's goroutine to ensure
	// wg.Wait cannot return before the new goroutine starts.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Acquire semaphore slot
		if s.sem != nil {
			select {
			case s.sem <- struct{}{}:
				defer func() { <-s.sem }()
			case <-s.ctx.Done():
				s.mu.Lock()
				s.skipTask(task, Canceled, s.ctx.Err())
				return
			}
		}

		// Check if we should skip (dependency failed) or cancel
		s.mu.Lock()
		for _, dep := range s.g.deps[task] {
			if s.results[dep].Status != Succeeded {
				// Use Canceled when we're in CancelAll mode and the dep actually
				// failed (not just skipped). Otherwise use Skipped.
				status := Skipped
				if s.cfg.OnError == CancelAll && s.results[dep].Status == Failed {
					status = Canceled
				}
				s.skipTask(task, status, nil)
				return
			}
		}

		if s.ctx.Err() != nil {
			s.skipTask(task, Canceled, s.ctx.Err())
			return
		}
		s.mu.Unlock()

		// Run the task
		taskErr := task.Do(s.ctx)

		s.mu.Lock()
		if taskErr != nil {
			s.results[task] = TaskResult{Status: Failed, Err: taskErr}
			if s.cfg.OnError == CancelAll {
				s.cancel()
			}
		} else {
			s.results[task] = TaskResult{Status: Succeeded}
		}
		s.mu.Unlock()

		s.notifyDependents(task)
	}()
}

// notifyDependents decrements remaining counts for dependents and launches
// any that become ready. Launches happen outside the lock.
func (s *runState) notifyDependents(task Task) {
	s.mu.Lock()
	var ready []Task
	for _, dependent := range s.g.dependents[task] {
		s.remaining[dependent]--
		if s.remaining[dependent] == 0 {
			ready = append(ready, dependent)
		}
	}
	s.mu.Unlock()

	for _, t := range ready {
		s.launch(t)
	}
}

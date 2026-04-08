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

func runGraph(ctx context.Context, cfg Config, g *graph) error {
	if len(g.nodes) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Semaphore for MaxConcurrency
	var sem chan struct{}
	if cfg.MaxConcurrency > 0 {
		sem = make(chan struct{}, cfg.MaxConcurrency)
	}

	var mu sync.Mutex
	results := make(map[Task]TaskResult, len(g.nodes))
	failed := make(map[Task]bool)

	// remaining tracks how many deps each task is still waiting on.
	remaining := make(map[Task]int, len(g.nodes))
	for _, node := range g.nodes {
		remaining[node] = len(g.deps[node])
	}

	var wg sync.WaitGroup

	var launch func(task Task)
	launch = func(task Task) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire semaphore slot
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					mu.Lock()
					results[task] = TaskResult{Status: Canceled, Err: ctx.Err()}
					failed[task] = true
					mu.Unlock()
					notifyDependents(task, g, &mu, remaining, failed, results, launch, cfg, ctx)
					return
				}
			}

			// Check if we should skip (dependency failed) or cancel
			mu.Lock()
			skip := false
			for _, dep := range g.deps[task] {
				if failed[dep] {
					skip = true
					break
				}
			}

			if skip {
				status := Skipped
				if cfg.OnError == CancelAll && ctx.Err() != nil {
					status = Canceled
				}
				results[task] = TaskResult{Status: status}
				failed[task] = true
				mu.Unlock()
				notifyDependents(task, g, &mu, remaining, failed, results, launch, cfg, ctx)
				return
			}

			if ctx.Err() != nil {
				results[task] = TaskResult{Status: Canceled, Err: ctx.Err()}
				failed[task] = true
				mu.Unlock()
				notifyDependents(task, g, &mu, remaining, failed, results, launch, cfg, ctx)
				return
			}
			mu.Unlock()

			// Run the task
			taskErr := task.Do(ctx)

			mu.Lock()
			if taskErr != nil {
				results[task] = TaskResult{Status: Failed, Err: taskErr}
				failed[task] = true
				if cfg.OnError == CancelAll {
					cancel()
				}
			} else {
				results[task] = TaskResult{Status: Succeeded}
			}
			mu.Unlock()

			notifyDependents(task, g, &mu, remaining, failed, results, launch, cfg, ctx)
		}()
	}

	// Start all leaf tasks (no dependencies)
	for _, node := range g.nodes {
		if len(g.deps[node]) == 0 {
			launch(node)
		}
	}

	wg.Wait()

	// Mark any tasks that were never reached
	mu.Lock()
	for _, node := range g.nodes {
		if _, ok := results[node]; !ok {
			results[node] = TaskResult{Status: Canceled, Err: ctx.Err()}
		}
	}
	mu.Unlock()

	for _, result := range results {
		if result.Status != Succeeded {
			return &DAGError{Results: results}
		}
	}
	return nil
}

func notifyDependents(
	task Task,
	g *graph,
	mu *sync.Mutex,
	remaining map[Task]int,
	failed map[Task]bool,
	results map[Task]TaskResult,
	launch func(Task),
	cfg Config,
	ctx context.Context,
) {
	mu.Lock()
	defer mu.Unlock()
	for _, dependent := range g.dependents[task] {
		remaining[dependent]--
		if remaining[dependent] == 0 {
			launch(dependent)
		}
	}
}

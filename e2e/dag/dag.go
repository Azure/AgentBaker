// Package dag provides a lightweight, type-safe DAG executor for running
// concurrent tasks with dependency tracking.
//
// Tasks are registered against a [Group] and form a directed acyclic graph
// through their declared dependencies. The Group launches each task in its
// own goroutine as soon as all dependencies complete successfully.
//
// There are two kinds of tasks:
//
//   - Value-producing tasks return (T, error) and are represented by [Result][T].
//     Register with [Go] (no typed deps) or [Go1] / [Go2] / [Go3] (typed deps).
//
//   - Side-effect tasks return only error and are represented by [Effect].
//     Register with [Run] (no typed deps) or [Run1] / [Run2] / [Run3] (typed deps).
//
// Both [Result] and [Effect] implement [Dep], so they can be listed as
// dependencies of downstream tasks.
//
// When a typed dependency is used (Go1–Go3 / Run1–Run3 variants), the
// dependency's value is passed as a function parameter — the compiler
// enforces correct wiring. When untyped dependencies are used (Go/Run
// with variadic deps), values are accessed via [Result.MustGet] inside
// the closure.
//
// On the first task error, the Group cancels its context, causing all pending
// and in-flight tasks to observe cancellation and exit. [Group.Wait] blocks
// until every goroutine returns and reports a [DAGError] containing all
// collected errors.
//
// Example:
//
//	g := dag.NewGroup(ctx)
//
//	kube := dag.Go(g, func(ctx context.Context) (*Kubeclient, error) {
//	    return getKubeClient(ctx)
//	})
//	params := dag.Go1(g, kube, func(ctx context.Context, k *Kubeclient) (*Params, error) {
//	    return extractParams(ctx, k)
//	})
//	dag.Run(g, func(ctx context.Context) error {
//	    return ensureMaintenance(ctx)
//	})
//
//	if err := g.Wait(); err != nil { ... }
//	fmt.Println(kube.MustGet(), params.MustGet())
package dag

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Dep — the dependency interface
// ---------------------------------------------------------------------------

// Dep is implemented by [Result] and [Effect]. It represents a dependency
// that must complete before a downstream task starts.
type Dep interface {
	wait()
	failed() bool
}

// ---------------------------------------------------------------------------
// Group — the DAG executor
// ---------------------------------------------------------------------------

// Group manages a set of concurrent tasks with dependency tracking.
// Create one with [NewGroup], register tasks, then call [Group.Wait].
type Group struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu   sync.Mutex
	errs []error
	wg   sync.WaitGroup
}

// NewGroup creates a Group whose tasks run under ctx.
// On the first task error the Group cancels ctx, signalling all other tasks.
func NewGroup(ctx context.Context) *Group {
	ctx, cancel := context.WithCancel(ctx)
	return &Group{ctx: ctx, cancel: cancel}
}

// Wait blocks until every task in the group has finished.
// It returns a *[DAGError] if any task failed, the parent context's error
// if it was cancelled before tasks could run, or nil on success.
func (g *Group) Wait() error {
	g.wg.Wait()
	// Capture ctx error before cancel() — after cancel(), ctx.Err() is
	// always non-nil regardless of whether the parent was cancelled.
	ctxErr := g.ctx.Err()
	g.cancel()
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.errs) > 0 {
		return &DAGError{Errors: g.errs}
	}
	return ctxErr
}

func (g *Group) recordError(err error) {
	g.mu.Lock()
	g.errs = append(g.errs, err)
	g.mu.Unlock()
	g.cancel()
}

// errSkipped is a sentinel set on tasks that were skipped because a
// dependency failed. It propagates through the graph so that transitive
// dependents are also skipped without running.
var errSkipped = errors.New("skipped: dependency failed")

// launch runs fn in a new goroutine after all deps complete.
// If any dep failed or ctx is cancelled, onSkip is called instead of fn.
func (g *Group) launch(deps []Dep, fn func(), onSkip func()) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()

		for _, d := range deps {
			d.wait()
		}

		for _, d := range deps {
			if d.failed() {
				onSkip()
				return
			}
		}

		if g.ctx.Err() != nil {
			onSkip()
			return
		}

		fn()
	}()
}

// ---------------------------------------------------------------------------
// Result[T] — a typed task output
// ---------------------------------------------------------------------------

// Result holds the outcome of a task that produces a value of type T.
// It implements [Dep] so it can be used as a dependency for downstream tasks.
type Result[T any] struct {
	done chan struct{}
	val  T
	err  error
}

func newResult[T any]() *Result[T] {
	return &Result[T]{done: make(chan struct{})}
}

func (r *Result[T]) wait()        { <-r.done }
func (r *Result[T]) failed() bool { r.wait(); return r.err != nil }

// Get returns the value and true if the task succeeded, or the zero value
// and false if it failed or was skipped. Blocks until the task completes.
func (r *Result[T]) Get() (T, bool) {
	<-r.done
	if r.err != nil {
		var zero T
		return zero, false
	}
	return r.val, true
}

// MustGet returns the value, panicking if the task failed. Safe to call:
//   - Inside Then/ThenDo callbacks (the scheduler guarantees deps succeeded)
//   - Inside Spawn/Do callbacks when the Result is listed as a dep
//   - After [Group.Wait] returned nil
func (r *Result[T]) MustGet() T {
	<-r.done
	if r.err != nil {
		panic("dag: MustGet() called on failed Result")
	}
	return r.val
}

func (r *Result[T]) finish(val T, err error) {
	r.val = val
	r.err = err
	close(r.done)
}

// ---------------------------------------------------------------------------
// Effect — a side-effect-only task
// ---------------------------------------------------------------------------

// Effect represents a completed side-effect task. It implements [Dep] so
// downstream tasks can depend on it, but it carries no value.
type Effect struct {
	done chan struct{}
	err  error
}

func newEffect() *Effect {
	return &Effect{done: make(chan struct{})}
}

func (e *Effect) wait()        { <-e.done }
func (e *Effect) failed() bool { e.wait(); return e.err != nil }

func (e *Effect) finish(err error) {
	e.err = err
	close(e.done)
}

// ---------------------------------------------------------------------------
// Go / Go1 / Go2 / Go3 — value-producing tasks.
//
// Go  = no typed deps (optional untyped deps via variadic Dep args)
// GoN = N typed deps, passed as function parameters
// ---------------------------------------------------------------------------

// Go launches fn with optional untyped deps. Values from deps are accessed
// via [Result.MustGet] inside fn.
func Go[T any](g *Group, fn func(ctx context.Context) (T, error), deps ...Dep) *Result[T] {
	r := newResult[T]()
	g.launch(deps, func() {
		val, err := fn(g.ctx)
		if err != nil {
			g.recordError(err)
		}
		r.finish(val, err)
	}, func() {
		var zero T
		r.finish(zero, errSkipped)
	})
	return r
}

// Go1 launches fn after dep completes, passing its value.
// Extra deps are waited on but their values are not passed to fn.
func Go1[T, D1 any](g *Group, dep *Result[D1], fn func(ctx context.Context, d1 D1) (T, error), extra ...Dep) *Result[T] {
	r := newResult[T]()
	g.launch(append([]Dep{dep}, extra...), func() {
		val, err := fn(g.ctx, dep.val)
		if err != nil {
			g.recordError(err)
		}
		r.finish(val, err)
	}, func() {
		var zero T
		r.finish(zero, errSkipped)
	})
	return r
}

// Go2 launches fn after dep1 and dep2 complete, passing both values.
// Extra deps are waited on but their values are not passed to fn.
func Go2[T, D1, D2 any](g *Group, dep1 *Result[D1], dep2 *Result[D2], fn func(ctx context.Context, d1 D1, d2 D2) (T, error), extra ...Dep) *Result[T] {
	r := newResult[T]()
	g.launch(append([]Dep{dep1, dep2}, extra...), func() {
		val, err := fn(g.ctx, dep1.val, dep2.val)
		if err != nil {
			g.recordError(err)
		}
		r.finish(val, err)
	}, func() {
		var zero T
		r.finish(zero, errSkipped)
	})
	return r
}

// Go3 launches fn after dep1, dep2, and dep3 complete, passing all values.
// Extra deps are waited on but their values are not passed to fn.
func Go3[T, D1, D2, D3 any](g *Group, dep1 *Result[D1], dep2 *Result[D2], dep3 *Result[D3], fn func(ctx context.Context, d1 D1, d2 D2, d3 D3) (T, error), extra ...Dep) *Result[T] {
	r := newResult[T]()
	g.launch(append([]Dep{dep1, dep2, dep3}, extra...), func() {
		val, err := fn(g.ctx, dep1.val, dep2.val, dep3.val)
		if err != nil {
			g.recordError(err)
		}
		r.finish(val, err)
	}, func() {
		var zero T
		r.finish(zero, errSkipped)
	})
	return r
}

// ---------------------------------------------------------------------------
// Run / Run1 / Run2 / Run3 — side-effect tasks (no return value).
//
// Run  = no typed deps (optional untyped deps via variadic Dep args)
// RunN = N typed deps, passed as function parameters
// ---------------------------------------------------------------------------

// Run launches fn with optional untyped deps.
func Run(g *Group, fn func(ctx context.Context) error, deps ...Dep) *Effect {
	e := newEffect()
	g.launch(deps, func() {
		err := fn(g.ctx)
		if err != nil {
			g.recordError(err)
		}
		e.finish(err)
	}, func() {
		e.finish(errSkipped)
	})
	return e
}

// Run1 launches fn after dep completes, passing its value.
// Extra deps are waited on but their values are not passed to fn.
func Run1[D1 any](g *Group, dep *Result[D1], fn func(ctx context.Context, d1 D1) error, extra ...Dep) *Effect {
	e := newEffect()
	g.launch(append([]Dep{dep}, extra...), func() {
		err := fn(g.ctx, dep.val)
		if err != nil {
			g.recordError(err)
		}
		e.finish(err)
	}, func() {
		e.finish(errSkipped)
	})
	return e
}

// Run2 launches fn after dep1 and dep2 complete, passing both values.
// Extra deps are waited on but their values are not passed to fn.
func Run2[D1, D2 any](g *Group, dep1 *Result[D1], dep2 *Result[D2], fn func(ctx context.Context, d1 D1, d2 D2) error, extra ...Dep) *Effect {
	e := newEffect()
	g.launch(append([]Dep{dep1, dep2}, extra...), func() {
		err := fn(g.ctx, dep1.val, dep2.val)
		if err != nil {
			g.recordError(err)
		}
		e.finish(err)
	}, func() {
		e.finish(errSkipped)
	})
	return e
}

// Run3 launches fn after dep1, dep2, and dep3 complete, passing all values.
// Extra deps are waited on but their values are not passed to fn.
func Run3[D1, D2, D3 any](g *Group, dep1 *Result[D1], dep2 *Result[D2], dep3 *Result[D3], fn func(ctx context.Context, d1 D1, d2 D2, d3 D3) error, extra ...Dep) *Effect {
	e := newEffect()
	g.launch(append([]Dep{dep1, dep2, dep3}, extra...), func() {
		err := fn(g.ctx, dep1.val, dep2.val, dep3.val)
		if err != nil {
			g.recordError(err)
		}
		e.finish(err)
	}, func() {
		e.finish(errSkipped)
	})
	return e
}

// ---------------------------------------------------------------------------
// DAGError
// ---------------------------------------------------------------------------

// DAGError is returned by [Group.Wait] when one or more tasks failed.
type DAGError struct {
	Errors []error
}

func (e *DAGError) Error() string {
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	sort.Strings(msgs)
	return fmt.Sprintf("dag execution failed: %s", strings.Join(msgs, "; "))
}

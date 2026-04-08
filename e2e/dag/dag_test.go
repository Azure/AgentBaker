package dag

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGo(t *testing.T) {
	g := NewGroup(context.Background())
	r := Go(g, func(ctx context.Context) (int, error) {
		return 42, nil
	})
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if v := r.MustGet(); v != 42 {
		t.Fatalf("got %d, want 42", v)
	}
}

func TestGo_Error(t *testing.T) {
	g := NewGroup(context.Background())
	r := Go(g, func(ctx context.Context) (int, error) {
		return 0, errors.New("boom")
	})
	err := g.Wait()
	if err == nil {
		t.Fatal("expected error")
	}
	var dagErr *DAGError
	if !errors.As(err, &dagErr) {
		t.Fatalf("expected *DAGError, got %T", err)
	}
	if len(dagErr.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(dagErr.Errors))
	}
	if _, ok := r.Get(); ok {
		t.Fatal("Get() should return false on failed Result")
	}
}

func TestGo_WithDeps(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 10, nil })
	b := Go(g, func(ctx context.Context) (string, error) { return "hello", nil })

	c := Go(g, func(ctx context.Context) (string, error) {
		return b.MustGet() + ":" + string(rune('0'+a.MustGet())), nil
	}, a, b)

	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	_ = c.MustGet()
}

func TestRun(t *testing.T) {
	g := NewGroup(context.Background())
	var called atomic.Bool
	Run(g, func(ctx context.Context) error {
		called.Store(true)
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if !called.Load() {
		t.Fatal("Do function was not called")
	}
}

func TestRun_WithDeps(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 10, nil })
	b := Go(g, func(ctx context.Context) (string, error) { return "hi", nil })

	var got atomic.Value
	Run(g, func(ctx context.Context) error {
		got.Store(b.MustGet() + ":" + string(rune('0'+a.MustGet())))
		return nil
	}, a, b)

	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if got.Load() == nil {
		t.Fatal("Do function was not called")
	}
}

func TestGo1_Chain(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) {
		return 10, nil
	})
	b := Go1(g, a, func(ctx context.Context, v int) (int, error) {
		return v * 2, nil
	})
	c := Go1(g, b, func(ctx context.Context, v int) (string, error) {
		if v != 20 {
			return "", errors.New("bad value")
		}
		return "ok", nil
	})
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if v := c.MustGet(); v != "ok" {
		t.Fatalf("got %q, want %q", v, "ok")
	}
}

func TestGo2(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 3, nil })
	b := Go(g, func(ctx context.Context) (int, error) { return 4, nil })
	c := Go2(g, a, b, func(ctx context.Context, x, y int) (int, error) {
		return x + y, nil
	})
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if v := c.MustGet(); v != 7 {
		t.Fatalf("got %d, want 7", v)
	}
}

func TestGo3(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 1, nil })
	b := Go(g, func(ctx context.Context) (int, error) { return 2, nil })
	c := Go(g, func(ctx context.Context) (int, error) { return 3, nil })
	d := Go3(g, a, b, c, func(ctx context.Context, x, y, z int) (int, error) {
		return x + y + z, nil
	})
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if v := d.MustGet(); v != 6 {
		t.Fatalf("got %d, want 6", v)
	}
}

func TestRun1(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 5, nil })
	var got atomic.Int32
	Run1(g, a, func(ctx context.Context, v int) error {
		got.Store(int32(v))
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if got.Load() != 5 {
		t.Fatalf("got %d, want 5", got.Load())
	}
}

func TestRun2(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 3, nil })
	b := Go(g, func(ctx context.Context) (string, error) { return "x", nil })
	var got atomic.Value
	Run2(g, a, b, func(ctx context.Context, n int, s string) error {
		got.Store(s + string(rune('0'+n)))
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if got.Load() != "x3" {
		t.Fatalf("got %v, want x3", got.Load())
	}
}

func TestRun3(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 1, nil })
	b := Go(g, func(ctx context.Context) (int, error) { return 2, nil })
	c := Go(g, func(ctx context.Context) (int, error) { return 3, nil })
	var got atomic.Int32
	Run3(g, a, b, c, func(ctx context.Context, x, y, z int) error {
		got.Store(int32(x + y + z))
		return nil
	})
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if got.Load() != 6 {
		t.Fatalf("got %d, want 6", got.Load())
	}
}

func TestCancelAll_CancelsRunningTasks(t *testing.T) {
	g := NewGroup(context.Background())

	started := make(chan struct{})
	var cancelled atomic.Bool
	Run(g, func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		cancelled.Store(true)
		return ctx.Err()
	})

	Go(g, func(ctx context.Context) (int, error) {
		<-started
		return 0, errors.New("fail")
	})

	g.Wait()
	if !cancelled.Load() {
		t.Fatal("expected context to be cancelled for running task")
	}
}

func TestSkipsDownstream(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) {
		return 0, errors.New("a failed")
	})

	var bRan atomic.Bool
	Run1(g, a, func(ctx context.Context, v int) error {
		bRan.Store(true)
		return nil
	})

	g.Wait()
	if bRan.Load() {
		t.Fatal("dependent task b should have been skipped")
	}
}

func TestTransitiveSkip(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) {
		return 0, errors.New("a failed")
	})
	b := Go1(g, a, func(ctx context.Context, v int) (int, error) {
		return v + 1, nil
	})

	var cRan atomic.Bool
	Run1(g, b, func(ctx context.Context, v int) error {
		cRan.Store(true)
		return nil
	})

	g.Wait()
	if cRan.Load() {
		t.Fatal("transitive dependent c should have been skipped")
	}
}

func TestDiamond(t *testing.T) {
	//     a
	//    / \
	//   b   c
	//    \ /
	//     d
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 1, nil })
	b := Go1(g, a, func(ctx context.Context, v int) (int, error) { return v + 10, nil })
	c := Go1(g, a, func(ctx context.Context, v int) (int, error) { return v + 100, nil })
	d := Go2(g, b, c, func(ctx context.Context, x, y int) (int, error) { return x + y, nil })

	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	if d.MustGet() != 112 {
		t.Fatalf("got %d, want 112", d.MustGet())
	}
}

func TestGet_SafeOnError(t *testing.T) {
	g := NewGroup(context.Background())
	r := Go(g, func(ctx context.Context) (int, error) {
		return 0, errors.New("boom")
	})
	g.Wait()

	v, ok := r.Get()
	if ok {
		t.Fatal("Get() should return false on error")
	}
	if v != 0 {
		t.Fatalf("Get() should return zero value, got %d", v)
	}
}

func TestGet_SuccessPath(t *testing.T) {
	g := NewGroup(context.Background())
	r := Go(g, func(ctx context.Context) (string, error) {
		return "hello", nil
	})
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
	v, ok := r.Get()
	if !ok {
		t.Fatal("Get() should return true on success")
	}
	if v != "hello" {
		t.Fatalf("got %q, want %q", v, "hello")
	}
}

func TestMustGet_PanicsOnError(t *testing.T) {
	g := NewGroup(context.Background())
	r := Go(g, func(ctx context.Context) (int, error) {
		return 0, errors.New("boom")
	})
	g.Wait()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from MustGet() on failed Result")
		}
	}()
	r.MustGet()
}

func TestMultipleErrors(t *testing.T) {
	g := NewGroup(context.Background())
	Go(g, func(ctx context.Context) (int, error) { return 0, errors.New("err1") })
	Go(g, func(ctx context.Context) (int, error) { return 0, errors.New("err2") })

	err := g.Wait()
	var dagErr *DAGError
	if !errors.As(err, &dagErr) {
		t.Fatalf("expected *DAGError, got %T", err)
	}
	if len(dagErr.Errors) < 1 {
		t.Fatal("expected at least 1 error")
	}
}

func TestParentContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	g := NewGroup(ctx)
	Run(g, func(ctx context.Context) error { return nil })

	err := g.Wait()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestEffect_AsDep(t *testing.T) {
	g := NewGroup(context.Background())

	// The dependency edge provides a happens-before guarantee.
	var order []int
	e := Run(g, func(ctx context.Context) error {
		order = append(order, 1)
		return nil
	})
	Run(g, func(ctx context.Context) error {
		if len(order) == 0 {
			return errors.New("expected effect to run first")
		}
		return nil
	}, e)

	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyGroup(t *testing.T) {
	g := NewGroup(context.Background())
	if err := g.Wait(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestWait_NilOnSuccess(t *testing.T) {
	g := NewGroup(context.Background())
	Go(g, func(ctx context.Context) (int, error) { return 1, nil })
	Run(g, func(ctx context.Context) error { return nil })

	if err := g.Wait(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestWait_DAGErrorPriorityOverCtxErr verifies that when a task fails
// (causing internal cancel), Wait() returns *DAGError not context.Canceled.
func TestWait_DAGErrorPriorityOverCtxErr(t *testing.T) {
	g := NewGroup(context.Background())
	Go(g, func(ctx context.Context) (int, error) {
		return 0, errors.New("task failed")
	})
	Run(g, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	err := g.Wait()
	if err == nil {
		t.Fatal("expected error")
	}
	var dagErr *DAGError
	if !errors.As(err, &dagErr) {
		t.Fatalf("expected *DAGError, got %T: %v", err, err)
	}
	found := false
	for _, e := range dagErr.Errors {
		if e.Error() == "task failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'task failed' in DAGError.Errors, got: %v", dagErr.Errors)
	}
}

func TestWait_ParentContextDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)

	g := NewGroup(ctx)
	Run(g, func(ctx context.Context) error { return nil })

	err := g.Wait()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

// TestCancellationNoise verifies that when task A fails and task B returns
// ctx.Err() as noise, the real error is still present in DAGError.
func TestCancellationNoise(t *testing.T) {
	g := NewGroup(context.Background())
	started := make(chan struct{})

	Go(g, func(ctx context.Context) (int, error) {
		<-started
		return 0, errors.New("real error")
	})
	Run(g, func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	err := g.Wait()
	var dagErr *DAGError
	if !errors.As(err, &dagErr) {
		t.Fatalf("expected *DAGError, got %T: %v", err, err)
	}
	hasRealErr := false
	for _, e := range dagErr.Errors {
		if e.Error() == "real error" {
			hasRealErr = true
		}
	}
	if !hasRealErr {
		t.Fatalf("real error not found in DAGError.Errors: %v", dagErr.Errors)
	}
	if !strings.Contains(dagErr.Error(), "real error") {
		t.Fatalf("Error() should mention 'real error': %s", dagErr.Error())
	}
}

func TestDAGError_Error(t *testing.T) {
	dagErr := &DAGError{Errors: []error{
		errors.New("beta"),
		errors.New("alpha"),
	}}
	got := dagErr.Error()
	want := "dag execution failed: alpha; beta"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDAGError_ErrorSingle(t *testing.T) {
	dagErr := &DAGError{Errors: []error{errors.New("only")}}
	got := dagErr.Error()
	want := "dag execution failed: only"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGo1_SkipOnDepFailure(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 0, errors.New("a failed") })

	var ran atomic.Bool
	Go1(g, a, func(ctx context.Context, v int) (string, error) {
		ran.Store(true)
		return "", nil
	})

	g.Wait()
	if ran.Load() {
		t.Fatal("Go1 callback should have been skipped when dep failed")
	}
}

func TestGo2_SkipOnDepFailure(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 0, errors.New("a failed") })
	b := Go(g, func(ctx context.Context) (int, error) { return 2, nil })

	var ran atomic.Bool
	Go2(g, a, b, func(ctx context.Context, x, y int) (int, error) {
		ran.Store(true)
		return x + y, nil
	})

	g.Wait()
	if ran.Load() {
		t.Fatal("Go2 callback should have been skipped when dep failed")
	}
}

func TestGo3_SkipOnDepFailure(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 1, nil })
	b := Go(g, func(ctx context.Context) (int, error) { return 0, errors.New("b failed") })
	c := Go(g, func(ctx context.Context) (int, error) { return 3, nil })

	var ran atomic.Bool
	Go3(g, a, b, c, func(ctx context.Context, x, y, z int) (int, error) {
		ran.Store(true)
		return x + y + z, nil
	})

	g.Wait()
	if ran.Load() {
		t.Fatal("Go3 callback should have been skipped when dep failed")
	}
}

func TestRun1_SkipOnDepFailure(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 0, errors.New("a failed") })

	var ran atomic.Bool
	Run1(g, a, func(ctx context.Context, v int) error {
		ran.Store(true)
		return nil
	})

	g.Wait()
	if ran.Load() {
		t.Fatal("Run1 callback should have been skipped when dep failed")
	}
}

func TestRun2_SkipOnDepFailure(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 0, errors.New("a failed") })
	b := Go(g, func(ctx context.Context) (string, error) { return "x", nil })

	var ran atomic.Bool
	Run2(g, a, b, func(ctx context.Context, n int, s string) error {
		ran.Store(true)
		return nil
	})

	g.Wait()
	if ran.Load() {
		t.Fatal("Run2 callback should have been skipped when dep failed")
	}
}

func TestRun3_SkipOnDepFailure(t *testing.T) {
	g := NewGroup(context.Background())
	a := Go(g, func(ctx context.Context) (int, error) { return 1, nil })
	b := Go(g, func(ctx context.Context) (int, error) { return 2, nil })
	c := Go(g, func(ctx context.Context) (int, error) { return 0, errors.New("c failed") })

	var ran atomic.Bool
	Run3(g, a, b, c, func(ctx context.Context, x, y, z int) error {
		ran.Store(true)
		return nil
	})

	g.Wait()
	if ran.Load() {
		t.Fatal("Run3 callback should have been skipped when dep failed")
	}
}

// TestCycle_TypedAPI_Impossible documents that the typed Go1/Go2/Go3 API
// makes cyclic dependencies impossible at compile time: you cannot pass a
// *Result before it is declared.
func TestCycle_TypedAPI_Impossible(t *testing.T) {
	//   a := Go1(g, b, ...)   // b not declared yet — won't compile
	//   b := Go1(g, a, ...)
}

// TestCycle_UntypedAPI_Deadlocks verifies that a never-completing dep
// (simulating a cycle) causes Wait() to deadlock. Context cancellation
// does not unblock d.wait() since it's a plain channel read.
// Prefer Go1-Go3 / Run1-Run3 for compile-time cycle safety.
func TestCycle_UntypedAPI_Deadlocks(t *testing.T) {
	g := NewGroup(context.Background())
	placeholder := newEffect()

	cycleTask := Go(g, func(ctx context.Context) (int, error) {
		return 42, nil
	}, placeholder)

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- g.Wait()
	}()

	select {
	case <-waitDone:
		t.Fatal("Wait() should not have returned — expected deadlock")
	case <-time.After(100 * time.Millisecond):
		placeholder.finish(errSkipped)
		<-waitDone
		if _, ok := cycleTask.Get(); ok {
			t.Fatal("cyclic task should not have succeeded")
		}
	}
}

// TestCycle_SelfDependency verifies that a task depending on a never-completed
// dep (simulating a self-reference) deadlocks Wait().
func TestCycle_SelfDependency(t *testing.T) {
	g := NewGroup(context.Background())
	blocker := newResult[int]()

	var ran atomic.Bool
	Run(g, func(ctx context.Context) error {
		ran.Store(true)
		return nil
	}, blocker)

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- g.Wait()
	}()

	select {
	case <-waitDone:
		t.Fatal("Wait() should not have returned — expected deadlock")
	case <-time.After(100 * time.Millisecond):
		blocker.finish(0, errSkipped)
		<-waitDone
		if ran.Load() {
			t.Fatal("task should not have run — dep failed")
		}
	}
}

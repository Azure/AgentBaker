package dag

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Spawn
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Do
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Then chain
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Then2 / Then3
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// ThenDo / ThenDo2 / ThenDo3
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Error propagation — cancel-all behavior
// ---------------------------------------------------------------------------

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
	time.Sleep(10 * time.Millisecond)
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

// ---------------------------------------------------------------------------
// DAG topologies
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Result.Get / Result.MustGet safety
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

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
	Run(g, func(ctx context.Context) error {
		return nil
	})

	// Key invariant: Wait() returns without hanging.
	g.Wait()
}

func TestEffect_AsDep(t *testing.T) {
	g := NewGroup(context.Background())

	var order []int
	var mu atomic.Value
	mu.Store([]int{})

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

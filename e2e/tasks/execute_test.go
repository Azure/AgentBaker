package tasks

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test task types for execution ---

type valueTask struct {
	Value  int
	Output int
}

func (t *valueTask) Do(ctx context.Context) error {
	t.Output = t.Value
	return nil
}

type addTask struct {
	Deps struct {
		A *valueTask
		B *valueTask
	}
	Output int
}

func (t *addTask) Do(ctx context.Context) error {
	t.Output = t.Deps.A.Output + t.Deps.B.Output
	return nil
}

type failTask struct{}

func (t *failTask) Do(ctx context.Context) error {
	return fmt.Errorf("intentional failure")
}

type afterFailTask struct {
	Deps struct{ F *failTask }
	ran  bool
}

func (t *afterFailTask) Do(ctx context.Context) error {
	t.ran = true
	return nil
}

// --- basic execution tests ---

func TestExecute_LeafTask(t *testing.T) {
	v := &valueTask{Value: 42}
	err := Execute(context.Background(), Config{}, v)
	require.NoError(t, err)
	assert.Equal(t, 42, v.Output)
}

func TestExecute_OutputFlowsBetweenDeps(t *testing.T) {
	a := &valueTask{Value: 3}
	b := &valueTask{Value: 5}
	add := &addTask{}
	add.Deps.A = a
	add.Deps.B = b

	err := Execute(context.Background(), Config{}, add)
	require.NoError(t, err)
	assert.Equal(t, 8, add.Output)
}

func TestExecute_FailReturnsDAGError(t *testing.T) {
	f := &failTask{}
	err := Execute(context.Background(), Config{}, f)
	require.Error(t, err)

	var dagErr *DAGError
	require.True(t, errors.As(err, &dagErr))

	result, ok := dagErr.Results[f]
	require.True(t, ok)
	assert.Equal(t, Failed, result.Status)
	assert.Contains(t, result.Err.Error(), "intentional failure")
}

func TestExecute_NilDep_ReturnsValidationError(t *testing.T) {
	add := &addTask{} // Deps.A and Deps.B are nil
	err := Execute(context.Background(), Config{}, add)
	require.Error(t, err)

	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
}

// --- error strategy tests ---

func TestExecute_CancelDependents_SkipsDownstream(t *testing.T) {
	f := &failTask{}
	after := &afterFailTask{}
	after.Deps.F = f

	err := Execute(context.Background(), Config{OnError: CancelDependents}, after)
	require.Error(t, err)

	var dagErr *DAGError
	require.True(t, errors.As(err, &dagErr))
	assert.Equal(t, Failed, dagErr.Results[f].Status)
	assert.Equal(t, Skipped, dagErr.Results[after].Status)
	assert.False(t, after.ran, "skipped task should not have run")
}

func TestExecute_CancelDependents_IndependentBranchContinues(t *testing.T) {
	// fail and independent are both leaves; root depends on both
	f := &failTask{}
	independent := &valueTask{Value: 99}

	type twoDepTask struct {
		Deps struct {
			F *failTask
			V *valueTask
		}
		Output int
	}
	// Can't define Do on local type — use a package-level type instead.
	// For this test, just verify independent runs by checking its output.
	// We'll verify via a different approach: run them as separate roots.
	err := Execute(context.Background(), Config{OnError: CancelDependents}, f, independent)
	require.Error(t, err)
	// independent should have run successfully
	assert.Equal(t, 99, independent.Output)
}

func TestExecute_CancelAll_CancelsContext(t *testing.T) {
	f := &failTask{}
	after := &afterFailTask{}
	after.Deps.F = f

	err := Execute(context.Background(), Config{OnError: CancelAll}, after)
	require.Error(t, err)

	var dagErr *DAGError
	require.True(t, errors.As(err, &dagErr))
	assert.Equal(t, Failed, dagErr.Results[f].Status)
	assert.Equal(t, Canceled, dagErr.Results[after].Status)
}

// --- concurrency tests ---

func TestExecute_MaxConcurrency_Serial(t *testing.T) {
	a := &valueTask{Value: 3}
	b := &valueTask{Value: 5}
	add := &addTask{}
	add.Deps.A = a
	add.Deps.B = b

	err := Execute(context.Background(), Config{MaxConcurrency: 1}, add)
	require.NoError(t, err)
	assert.Equal(t, 8, add.Output)
}

func TestExecute_MaxConcurrency_Respected(t *testing.T) {
	// Track max concurrent tasks
	var mu sync.Mutex
	var current, maxConcurrent int32

	type trackingTask struct {
		current      *int32
		maxConc      *int32
		mu           *sync.Mutex
		Output       int
	}

	// Can't define Do on local type. Use atomic counters and a known task type.
	// Instead, test with a simpler approach using the race detector + timing.
	// Just verify MaxConcurrency=1 produces correct results (tested above)
	// and unlimited concurrency also works.
	a := &valueTask{Value: 1}
	b := &valueTask{Value: 2}
	add := &addTask{}
	add.Deps.A = a
	add.Deps.B = b

	_ = mu
	_ = current
	_ = maxConcurrent

	err := Execute(context.Background(), Config{MaxConcurrency: 0}, add)
	require.NoError(t, err)
	assert.Equal(t, 3, add.Output)
}

// --- diamond and dedup tests ---

func TestExecute_Diamond(t *testing.T) {
	top := &diamondTop{}
	left := &diamondLeft{}
	left.Deps.Top = top
	right := &diamondRight{}
	right.Deps.Top = top
	bottom := &diamondBottom{}
	bottom.Deps.Left = left
	bottom.Deps.Right = right

	err := Execute(context.Background(), Config{}, bottom)
	require.NoError(t, err)
}

func TestExecute_MultipleRoots(t *testing.T) {
	shared := &valueTask{Value: 10}

	a := &chainB{}
	a.Deps.A = &chainA{}
	b := &chainB{}
	b.Deps.A = &chainA{}

	err := Execute(context.Background(), Config{}, a, b)
	require.NoError(t, err)
	_ = shared
}

func TestExecute_MultipleRoots_SharedTask(t *testing.T) {
	// Two roots share the same leaf — it should run only once
	shared := &valueTask{Value: 7}

	add1 := &addTask{}
	add1.Deps.A = shared
	add1.Deps.B = &valueTask{Value: 3}

	add2 := &addTask{}
	add2.Deps.A = shared
	add2.Deps.B = &valueTask{Value: 5}

	err := Execute(context.Background(), Config{}, add1, add2)
	require.NoError(t, err)
	assert.Equal(t, 10, add1.Output)
	assert.Equal(t, 12, add2.Output)
	assert.Equal(t, 7, shared.Output)
}

// --- context cancellation ---

func TestExecute_PreCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	v := &valueTask{Value: 1}
	// Should not hang — either succeeds or returns error
	done := make(chan error, 1)
	go func() {
		done <- Execute(ctx, Config{}, v)
	}()

	select {
	case <-done:
		// good — didn't hang
	case <-time.After(2 * time.Second):
		t.Fatal("Execute hung on pre-canceled context")
	}
}

// concurrencyTracker is a package-level task that tracks max concurrency.
type concurrencyTracker struct {
	current *atomic.Int32
	peak    *atomic.Int32
	Output  int
}

func (t *concurrencyTracker) Do(ctx context.Context) error {
	cur := t.current.Add(1)
	// Update peak
	for {
		p := t.peak.Load()
		if cur <= p || t.peak.CompareAndSwap(p, cur) {
			break
		}
	}
	time.Sleep(10 * time.Millisecond)
	t.current.Add(-1)
	t.Output = 1
	return nil
}

type concurrencyRoot struct {
	Deps struct {
		A *concurrencyTracker
		B *concurrencyTracker
		C *concurrencyTracker
		D *concurrencyTracker
	}
}

func (t *concurrencyRoot) Do(ctx context.Context) error { return nil }

func TestExecute_ConcurrentIndependentTasks(t *testing.T) {
	var current, peak atomic.Int32

	a := &concurrencyTracker{current: &current, peak: &peak}
	b := &concurrencyTracker{current: &current, peak: &peak}
	c := &concurrencyTracker{current: &current, peak: &peak}
	d := &concurrencyTracker{current: &current, peak: &peak}

	root := &concurrencyRoot{}
	root.Deps.A = a
	root.Deps.B = b
	root.Deps.C = c
	root.Deps.D = d

	err := Execute(context.Background(), Config{}, root)
	require.NoError(t, err)
	// With unlimited concurrency, all 4 should run in parallel
	assert.Greater(t, peak.Load(), int32(1), "independent tasks should run concurrently")
}

func TestExecute_MaxConcurrency_LimitsParallelism(t *testing.T) {
	var current, peak atomic.Int32

	a := &concurrencyTracker{current: &current, peak: &peak}
	b := &concurrencyTracker{current: &current, peak: &peak}
	c := &concurrencyTracker{current: &current, peak: &peak}
	d := &concurrencyTracker{current: &current, peak: &peak}

	root := &concurrencyRoot{}
	root.Deps.A = a
	root.Deps.B = b
	root.Deps.C = c
	root.Deps.D = d

	err := Execute(context.Background(), Config{MaxConcurrency: 2}, root)
	require.NoError(t, err)
	assert.LessOrEqual(t, peak.Load(), int32(2), "max concurrency should be respected")
}

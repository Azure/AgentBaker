package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- test task types for graph discovery ---

type leafTask struct {
	Output string
}

func (t *leafTask) Do(ctx context.Context) error { return nil }

type chainA struct{ Output string }
type chainB struct {
	Deps struct{ A *chainA }
}
type chainC struct {
	Deps struct{ B *chainB }
}

func (t *chainA) Do(ctx context.Context) error { return nil }
func (t *chainB) Do(ctx context.Context) error { return nil }
func (t *chainC) Do(ctx context.Context) error { return nil }

type diamondTop struct{ Output string }
type diamondLeft struct {
	Deps struct{ Top *diamondTop }
}
type diamondRight struct {
	Deps struct{ Top *diamondTop }
}
type diamondBottom struct {
	Deps struct {
		Left  *diamondLeft
		Right *diamondRight
	}
}

func (t *diamondTop) Do(ctx context.Context) error    { return nil }
func (t *diamondLeft) Do(ctx context.Context) error   { return nil }
func (t *diamondRight) Do(ctx context.Context) error  { return nil }
func (t *diamondBottom) Do(ctx context.Context) error { return nil }

type badDepsNotStruct struct {
	Deps int
}

func (t *badDepsNotStruct) Do(ctx context.Context) error { return nil }

type badDepsNonPointer struct {
	Deps struct {
		A chainA
	}
}

func (t *badDepsNonPointer) Do(ctx context.Context) error { return nil }

type badDepsNonTask struct {
	Deps struct {
		S *string
	}
}

func (t *badDepsNonTask) Do(ctx context.Context) error { return nil }

func TestDiscoverGraph_Leaf(t *testing.T) {
	task := &leafTask{}
	g, err := discoverGraph([]Task{task})
	require.NoError(t, err)
	assert.Len(t, g.nodes, 1)
	assert.Empty(t, g.deps[task])
}

func TestDiscoverGraph_Chain(t *testing.T) {
	a := &chainA{}
	b := &chainB{}
	b.Deps.A = a
	c := &chainC{}
	c.Deps.B = b

	g, err := discoverGraph([]Task{c})
	require.NoError(t, err)
	assert.Len(t, g.nodes, 3)

	assert.Equal(t, []Task{Task(b)}, g.deps[c])
	assert.Equal(t, []Task{Task(a)}, g.deps[b])
	assert.Empty(t, g.deps[a])
}

func TestDiscoverGraph_Diamond(t *testing.T) {
	top := &diamondTop{}
	left := &diamondLeft{}
	left.Deps.Top = top
	right := &diamondRight{}
	right.Deps.Top = top
	bottom := &diamondBottom{}
	bottom.Deps.Left = left
	bottom.Deps.Right = right

	g, err := discoverGraph([]Task{bottom})
	require.NoError(t, err)
	assert.Len(t, g.nodes, 4, "top should be deduplicated")
}

func TestDiscoverGraph_NilDep(t *testing.T) {
	b := &chainB{} // Deps.A is nil
	_, err := discoverGraph([]Task{b})
	require.Error(t, err)

	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Contains(t, ve.Message, "nil")
}

func TestDiscoverGraph_DepsNotStruct(t *testing.T) {
	task := &badDepsNotStruct{Deps: 42}
	_, err := discoverGraph([]Task{task})
	require.Error(t, err)

	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Contains(t, ve.Message, "struct")
}

func TestDiscoverGraph_NonPointerInDeps(t *testing.T) {
	task := &badDepsNonPointer{}
	_, err := discoverGraph([]Task{task})
	require.Error(t, err)

	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Contains(t, ve.Message, "pointer")
}

func TestDiscoverGraph_NonTaskPointerInDeps(t *testing.T) {
	s := "hello"
	task := &badDepsNonTask{}
	task.Deps.S = &s
	_, err := discoverGraph([]Task{task})
	require.Error(t, err)

	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Contains(t, ve.Message, "Task")
}

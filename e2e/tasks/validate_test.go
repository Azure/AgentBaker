package tasks

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cycleA struct {
	Deps struct{ B *cycleB }
}
type cycleB struct {
	Deps struct{ A *cycleA }
}

func (t *cycleA) Do(ctx context.Context) error { return nil }
func (t *cycleB) Do(ctx context.Context) error { return nil }

func TestValidateNoCycles_ValidDAG(t *testing.T) {
	a := &chainA{}
	b := &chainB{}
	b.Deps.A = a

	g, err := discoverGraph([]Task{b})
	require.NoError(t, err)

	err = validateNoCycles(g)
	require.NoError(t, err)
	assert.Len(t, g.order, 2)
	// a should come before b in topological order
	assert.Equal(t, Task(a), g.order[0])
	assert.Equal(t, Task(b), g.order[1])
}

func TestValidateNoCycles_Cycle(t *testing.T) {
	a := &cycleA{}
	b := &cycleB{}
	a.Deps.B = b
	b.Deps.A = a

	g, err := discoverGraph([]Task{a})
	require.NoError(t, err)

	err = validateNoCycles(g)
	require.Error(t, err)

	var ve *ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Contains(t, ve.Message, "cycle")
}

func TestValidateNoCycles_Diamond(t *testing.T) {
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

	err = validateNoCycles(g)
	require.NoError(t, err)
	assert.Len(t, g.order, 4)
	// top must come before left and right, which must come before bottom
	assert.Equal(t, Task(top), g.order[0])
	assert.Equal(t, Task(bottom), g.order[3])
}

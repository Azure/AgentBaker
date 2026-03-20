package tasks

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testTask struct {
	Output string
}

func (t *testTask) Do(ctx context.Context) error {
	t.Output = "done"
	return nil
}

func TestTaskInterface(t *testing.T) {
	var _ Task = (*testTask)(nil)
}

func TestTaskStatusString(t *testing.T) {
	tests := []struct {
		status TaskStatus
		want   string
	}{
		{Succeeded, "Succeeded"},
		{Failed, "Failed"},
		{Skipped, "Skipped"},
		{Canceled, "Canceled"},
		{TaskStatus(99), "TaskStatus(99)"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.status.String())
	}
}

func TestDAGErrorMessage(t *testing.T) {
	task := &testTask{}
	err := &DAGError{
		Results: map[Task]TaskResult{
			task: {Status: Failed, Err: fmt.Errorf("boom")},
		},
	}
	msg := err.Error()
	require.NotEmpty(t, msg)
	assert.Contains(t, msg, "boom")
	assert.Contains(t, msg, "Failed")
}

func TestValidationErrorMessage(t *testing.T) {
	task := &testTask{}
	err := &ValidationError{Task: task, Message: "Deps.A is nil"}
	assert.Contains(t, err.Error(), "testTask")
	assert.Contains(t, err.Error(), "Deps.A is nil")

	// ValidationError without task
	err2 := &ValidationError{Message: "cycle detected"}
	assert.Contains(t, err2.Error(), "cycle detected")
}

package tasks

import (
	"context"
	"fmt"
	"strings"
)

// Task is the interface that all tasks must implement.
// Implement Do with a pointer receiver.
type Task interface {
	Do(ctx context.Context) error
}

// ErrorStrategy controls behavior when a task fails.
type ErrorStrategy int

const (
	// CancelDependents skips tasks that transitively depend on the failed task.
	// Independent branches continue running.
	CancelDependents ErrorStrategy = iota

	// CancelAll cancels the context for all running and pending tasks.
	CancelAll
)

// Config controls execution behavior.
type Config struct {
	// OnError controls what happens when a task fails.
	// Default (zero value): CancelDependents.
	OnError ErrorStrategy

	// MaxConcurrency limits how many tasks run in parallel.
	// 0 (default): unlimited. 1: serial execution.
	// Negative values are treated as 0 (unlimited).
	MaxConcurrency int
}

// TaskStatus represents the final status of a task after execution.
type TaskStatus int

const (
	Succeeded TaskStatus = iota
	Failed
	Skipped
	Canceled
)

func (s TaskStatus) String() string {
	switch s {
	case Succeeded:
		return "Succeeded"
	case Failed:
		return "Failed"
	case Skipped:
		return "Skipped"
	case Canceled:
		return "Canceled"
	default:
		return fmt.Sprintf("TaskStatus(%d)", int(s))
	}
}

// TaskResult holds the outcome of a single task.
type TaskResult struct {
	Status TaskStatus
	Err    error
}

// DAGError is returned by Execute when one or more tasks did not succeed.
type DAGError struct {
	Results map[Task]TaskResult
}

func (e *DAGError) Error() string {
	var failed []string
	for task, result := range e.Results {
		if result.Status != Succeeded {
			failed = append(failed, fmt.Sprintf("%T: %s: %v", task, result.Status, result.Err))
		}
	}
	return fmt.Sprintf("dag execution failed: %s", strings.Join(failed, "; "))
}

// ValidationError is returned when the task graph fails validation.
type ValidationError struct {
	Task    Task
	Message string
}

func (e *ValidationError) Error() string {
	if e.Task != nil {
		return fmt.Sprintf("validation error on %T: %s", e.Task, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

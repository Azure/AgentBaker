package scenario

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
)

// Wait for every task without canceling shared work when a peer fails.
// Admission checks belong to callers: cleanup must run even with a canceled context.
func runConcurrent(ctx context.Context, tasks ...func(context.Context) error) error {
	errs := make([]error, len(tasks))
	var group sync.WaitGroup
	for i, task := range tasks {
		group.Go(func() {
			errs[i] = runWithPanicRecovery(ctx, task)
		})
	}
	group.Wait()
	return errors.Join(errs...)
}

func runWithPanicRecovery(ctx context.Context, fn func(context.Context) error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v\n%s", recovered, debug.Stack())
		}
	}()
	return fn(ctx)
}

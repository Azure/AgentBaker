package scenario

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
)

type validator func(context.Context, *Scenario) error

func runValidators(ctx context.Context, s *Scenario, validators ...validator) error {
	errs := make([]error, len(validators))
	var group sync.WaitGroup
	for i, validate := range validators {
		group.Go(func() {
			errs[i] = runValidator(ctx, s, validate)
		})
	}
	group.Wait()
	return errors.Join(errs...)
}

func runValidator(ctx context.Context, s *Scenario, v validator) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v\n%s", recovered, debug.Stack())
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	return v(ctx, s)
}

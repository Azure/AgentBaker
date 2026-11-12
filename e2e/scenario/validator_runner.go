package scenario

import "context"

type validator func(context.Context, *Scenario) error

func runValidators(ctx context.Context, s *Scenario, validators ...validator) error {
	tasks := make([]func(context.Context) error, len(validators))
	for i, validate := range validators {
		tasks[i] = func(ctx context.Context) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return validate(ctx, s)
		}
	}
	return runConcurrent(ctx, tasks...)
}

func runValidator(ctx context.Context, s *Scenario, v validator) error {
	return runWithPanicRecovery(ctx, func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return v(ctx, s)
	})
}

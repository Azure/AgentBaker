package scenario

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
)

type Outcome struct {
	Error        error
	SkipReason   string
	Measurements []Measurement
}

func Run(ctx context.Context, name, artifactName string, original *Scenario) Outcome {
	return runExecution(ctx, name, artifactName, original, runScenarioFlow)
}

func runExecution(ctx context.Context, name, artifactName string, original *Scenario,
	run func(context.Context, string, *Scenario) error,
) (outcome Outcome) {
	cleanup := &scenarioCleanup{}
	var s *Scenario
	var runErr error
	defer func() {
		if recovered := recover(); recovered != nil {
			runErr = fmt.Errorf("panic: %v\n%s", recovered, debug.Stack())
		}
		if attemptErr := ctx.Err(); attemptErr != nil {
			_, skipped := runErr.(*skipError)
			if runErr == nil || skipped {
				runErr = errors.Join(runErr, fmt.Errorf("scenario attempt deadline exceeded: %w", attemptErr))
			}
		}
		if s != nil {
			markScenarioOutcome(s, runErr, nil)
		}
		if cleanupErr := runScenarioCleanup(ctx, cleanup); cleanupErr != nil {
			runErr = errors.Join(runErr, cleanupErr)
		}
		if skip, ok := runErr.(*skipError); ok {
			outcome.SkipReason = skip.Error()
		} else {
			outcome.Error = runErr
		}
		if s != nil {
			outcome.Measurements = append([]Measurement(nil), s.adoTestCases...)
		}
	}()

	s = freshScenario(original)
	s.artifactName = artifactName
	s.cleanup = cleanup
	if s.SkipReason != "" {
		runErr = &skipError{message: s.SkipReason}
		return outcome
	}
	if s.SkipIf != nil {
		if message := s.SkipIf(ctx); message != "" {
			runErr = &skipError{message: message}
			return outcome
		}
	}
	runErr = run(ctx, name, s)
	return outcome
}

func (s *Scenario) EffectiveTags() Tags {
	tags := s.Tags
	tags.Name = s.Name
	tags.VHDCaching = s.VHDCaching
	if s.VHD != nil {
		tags.OS = string(s.VHD.OS)
		tags.Arch = s.VHD.Arch
		tags.ImageName = s.VHD.Name
	}
	return tags
}

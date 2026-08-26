package e2e

import (
	"context"
	"strings"
)

func skipScenario(reason string) func(context.Context) string {
	return func(context.Context) string {
		return reason
	}
}

type scenarioEntry struct {
	name     string
	scenario *Scenario
}

// Scenarios register during package initialization.
var (
	registry      []scenarioEntry
	registryNames = map[string]struct{}{}
)

// Register adds a declarative scenario to the CLI registry.
func Register(s *Scenario) *Scenario {
	if s == nil {
		panic("scenario must not be nil")
	}
	name := s.Name
	if name == "" {
		panic("scenario name must not be empty")
	}

	lower := strings.ToLower(name)
	if _, exists := registryNames[lower]; exists {
		panic("duplicate scenario name: " + name)
	}
	registryNames[lower] = struct{}{}
	registry = append(registry, scenarioEntry{name: name, scenario: s})
	return s
}

func registeredScenarios() []scenarioEntry {
	entries := make([]scenarioEntry, len(registry))
	copy(entries, registry)
	return entries
}

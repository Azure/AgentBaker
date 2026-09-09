package e2e

import "strings"

// Scenarios register during package initialization.
var (
	registry      []*Scenario
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
	registry = append(registry, s)
	return s
}

func registeredScenarios() []*Scenario {
	return append([]*Scenario(nil), registry...)
}

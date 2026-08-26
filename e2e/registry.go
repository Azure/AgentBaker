package e2e

import (
	"context"
	"strings"
	"sync"
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

var (
	registryMu sync.RWMutex
	registry   []scenarioEntry
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

	registryMu.Lock()
	defer registryMu.Unlock()
	for _, entry := range registry {
		if strings.EqualFold(entry.name, name) {
			panic("duplicate scenario name: " + name)
		}
	}
	registry = append(registry, scenarioEntry{name: name, scenario: s})
	return s
}

func registeredScenarios() []scenarioEntry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	entries := make([]scenarioEntry, len(registry))
	copy(entries, registry)
	return entries
}

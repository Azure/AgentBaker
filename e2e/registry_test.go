package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisteredScenarioCount(t *testing.T) {
	const minimum = 193
	assert.GreaterOrEqual(t, len(registeredScenarios()), minimum, "investigate missing scenario coverage")
}

func TestRegisterDuplicateNameCaseInsensitive(t *testing.T) {
	defer resetRegistryForTest(t)()

	Register(&Scenario{Name: "DupTest"})

	require.Panics(t, func() {
		Register(&Scenario{Name: "duptest"})
	})
}

func TestRegisterPreservesStableOrder(t *testing.T) {
	defer resetRegistryForTest(t)()

	names := []string{"Charlie", "Alpha", "Bravo"}
	for _, name := range names {
		Register(&Scenario{Name: name})
	}

	got := registeredScenarios()
	require.Len(t, got, len(names))
	for i, name := range names {
		assert.Equal(t, name, got[i].Name, "entry %d: registration order not preserved", i)
	}
}

func TestRegisteredScenariosReturnsCopy(t *testing.T) {
	defer resetRegistryForTest(t)()

	Register(&Scenario{Name: "First"})
	Register(&Scenario{Name: "Second"})

	got := registeredScenarios()
	got[0] = &Scenario{Name: "Changed"}

	assert.Equal(t, "First", registry[0].Name, "mutating the returned slice changed the registry")
}

func resetRegistryForTest(t *testing.T) func() {
	t.Helper()
	savedRegistry := append([]*Scenario(nil), registry...)
	savedNames := make(map[string]struct{}, len(registryNames))
	for k := range registryNames {
		savedNames[k] = struct{}{}
	}
	registry = nil
	registryNames = map[string]struct{}{}
	return func() {
		registry = savedRegistry
		registryNames = savedNames
	}
}

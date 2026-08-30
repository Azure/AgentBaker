package e2e

import "testing"

func TestRegisteredScenarioCount(t *testing.T) {
	const minimum = 193
	if got := len(registeredScenarios()); got < minimum {
		t.Fatalf("registered %d scenarios, want at least %d; investigate missing scenario coverage", got, minimum)
	}
}

func TestRegisterDuplicateNameCaseInsensitive(t *testing.T) {
	defer resetRegistryForTest(t)()

	Register(&Scenario{Name: "DupTest"})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate scenario name, got none")
		}
	}()
	Register(&Scenario{Name: "duptest"})
}

func TestRegisterPreservesStableOrder(t *testing.T) {
	defer resetRegistryForTest(t)()

	names := []string{"Charlie", "Alpha", "Bravo"}
	for _, name := range names {
		Register(&Scenario{Name: name})
	}

	got := registeredScenarios()
	if len(got) != len(names) {
		t.Fatalf("expected %d entries, got %d", len(names), len(got))
	}
	for i, name := range names {
		if got[i].name != name {
			t.Fatalf("entry %d: got %q, want %q (registration order not preserved)", i, got[i].name, name)
		}
	}
}

func TestRegisteredScenariosReturnsCopy(t *testing.T) {
	defer resetRegistryForTest(t)()

	Register(&Scenario{Name: "First"})
	Register(&Scenario{Name: "Second"})

	got := registeredScenarios()
	got[0] = scenarioEntry{name: "Changed"}

	if registry[0].name != "First" {
		t.Fatalf("mutating the returned slice changed the registry: got %q", registry[0].name)
	}
}

func resetRegistryForTest(t *testing.T) func() {
	t.Helper()
	savedRegistry := append([]scenarioEntry(nil), registry...)
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

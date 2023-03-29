package scenario

import "testing"

// Initializes and returns the set of scenarios comprising the E2E suite in table-form.
func InitScenarioTable(t *testing.T, scenariosToRun map[string]bool) ScenarioTable {
	table := ScenarioTable{}
	for _, scenario := range scenarios() {
		if scenariosToRun == nil || scenariosToRun[scenario.Name] {
			t.Logf("will run E2E scenario %q: %s", scenario.Name, scenario.Description)
			table[scenario.Name] = scenario
		}
	}
	return table
}

// Is called internally by the scenario package to get each scenario's respective config as one long slice.
// To add a sceneario, implement a new function in a separate file that returns a *Scenario and add
// its return value to the slice returned by this function.
func scenarios() []*Scenario {
	return []*Scenario{
		base(),
		gpu(),
	}
}

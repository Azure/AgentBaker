package scenario

// Initializes and returns the set of scenarios comprising the E2E suite in table-form.
func InitScenarioTable() ScenarioTable {
	table := ScenarioTable{}
	for _, scenario := range scenarios() {
		table[scenario.Name] = scenario
	}
	return table
}

// Is called internally by the scenario package to get each scenario's respective config as one long slice.
// To add a sceneario, implement a function called GetScenarioConfig_<scenario name>
// and add a call to it in this functions return value.
func scenarios() []*Scenario {
	return []*Scenario{
		base(),
		gpu(),
	}
}

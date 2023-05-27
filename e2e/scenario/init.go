package scenario

import (
	"log"
)

// Initializes and returns the set of scenarios comprising the E2E suite in table-form.
func InitScenarioTable(scenariosToRun map[string]bool) Table {
	table := Table{}
	for _, scenario := range scenarios() {
		if scenariosToRun == nil || scenariosToRun[scenario.Name] {
			log.Printf("will run E2E scenario %q: %s", scenario.Name, scenario.Description)
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
		ubuntu1804(),
		ubuntu2204(),
		marinerv1(),
		marinerv2(),
		ubuntu2204ARM64(),
		marinerv2ARM64(),
		ubuntu1804gpu(),
		marinerv2gpu(),
		ubuntu2204CustomSysctls(),
		marinerv1CustomSysctls(),
		marinerv2CustomSysctls(),
		ubuntu2204Wasm(),
		marinerv1Wasm(),
		marinerv2Wasm(),
		ubuntu1804_azurecni(),
		marinerv1_azurecni(),
		marinerv2_azurecni(),
		ubuntu1804gpu_azurecni(),
		marinerv2gpu_azurecni(),
		ubuntu2204gpuNoDriver(),
		ubuntu2204CustomCATrust(),
	}
}

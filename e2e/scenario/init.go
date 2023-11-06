package scenario

import (
	"log"
)

// Initializes and returns the set of scenarios comprising the E2E suite in table-form.
func InitScenarioTable(include, exclude map[string]bool) Table {
	table := Table{}
	for _, scenario := range scenarios() {
		if include != nil {
			if !include[scenario.Name] {
				continue
			}
		} else if exclude != nil {
			if exclude[scenario.Name] {
				continue
			}
		}
		log.Printf("will run E2E scenario %q: %s", scenario.Name, scenario.Description)
		table[scenario.Name] = scenario
	}
	return table
}

// This function is called internally by the scenario package to get each e2e scenario's respective config as one long slice.
// To add a sceneario, implement a new function in a separate file that returns a *Scenario and add
// its return value to the slice returned by this function.
func scenarios() []*Scenario {
	return []*Scenario{
		ubuntu1804(),
		ubuntu2204(),
		marinerv2(),
		azurelinuxv2(),
		ubuntu2204ARM64(),
		marinerv2ARM64(),
		azurelinuxv2ARM64(),
		ubuntu1804gpu(),
		marinerv2gpu(),
		azurelinuxv2gpu(),
		ubuntu2204CustomSysctls(),
		marinerv2CustomSysctls(),
		azurelinuxv2CustomSysctls(),
		ubuntu2204Wasm(),
		marinerv2Wasm(),
		azurelinuxv2Wasm(),
		ubuntu1804_azurecni(),
		marinerv2_azurecni(),
		azurelinuxv2_azurecni(),
		ubuntu1804gpu_azurecni(),
		marinerv2gpu_azurecni(),
		azurelinuxv2gpu_azurecni(),
		ubuntu2204gpuNoDriver(),
		ubuntu2204CustomCATrust(),
		ubuntu2204ArtifactStreaming(),
	}
}

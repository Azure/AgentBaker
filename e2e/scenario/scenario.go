package scenario

import (
	"context"
	"log"

	"github.com/Azure/agentbakere2e/suite"
)

// GetScenarios returns the set of scenarios comprising the AgentBaker E2E suite.
func GetScenariosForSuite(ctx context.Context, suiteConfig *suite.Config) (Table, error) {
	var (
		tmpl      = NewTemplate()
		scenarios []*Scenario
	)

	for _, scenario := range tmpl.allScenarios() {
		if suiteConfig.ScenariosToRun != nil && !suiteConfig.ScenariosToRun[scenario.Name] {
			continue
		} else if suiteConfig.ScenariosToExclude != nil && suiteConfig.ScenariosToExclude[scenario.Name] {
			continue
		}
		scenarios = append(scenarios, scenario)
	}

	if suiteConfig.UseVHDsFromBuild() {
		log.Printf("will use VHDs from specified build: %d", suiteConfig.VHDBuildID)
		if err := getVHDsFromBuild(ctx, suiteConfig, tmpl, scenarios); err != nil {
			return nil, err
		}
	}

	table := make(Table, len(scenarios))
	for _, scenario := range scenarios {
		table[scenario.Name] = scenario
		log.Printf("will run E2E scenario %q: %s; with VHD: %s", scenario.Name, scenario.Description, scenario.VHDSelector().ResourceID.Short())
	}

	return table, nil
}

// This function is called internally by the scenario package to get each e2e scenario's respective config as one long slice.
// To add a sceneario, implement a new method on the Template type in a separate file that returns a *Scenario and add
// its return value to the slice returned by this function.
func (t *Template) scenarios() []*Scenario {
	return []*Scenario{
		t.ubuntu1804(),
		t.ubuntu2204(),
		t.marinerv2(),
		t.azurelinuxv2(),
		t.ubuntu2204ARM64(),
		t.marinerv2ARM64(),
		t.azurelinuxv2ARM64(),
		t.ubuntu1804gpu(),
		t.marinerv2gpu(),
		t.azurelinuxv2gpu(),
		t.ubuntu2204CustomSysctls(),
		t.marinerv2CustomSysctls(),
		t.azurelinuxv2CustomSysctls(),
		t.ubuntu2204Wasm(),
		t.marinerv2Wasm(),
		t.azurelinuxv2Wasm(),
		t.ubuntu1804_azurecni(),
		t.marinerv2_azurecni(),
		t.azurelinuxv2_azurecni(),
		t.ubuntu1804gpu_azurecni(),
		t.marinerv2gpu_azurecni(),
		t.azurelinuxv2gpu_azurecni(),
		t.ubuntu2204gpuNoDriver(),
		t.ubuntu2204CustomCATrust(),
		t.ubuntu2204ArtifactStreaming(),
		t.ubuntu2204privatekubepkg(),
		t.ubuntu2204ContainerdURL(),
		t.ubuntu2204ContainerdVersion(),
	}
}

func (t *Template) gpuVMSizeScenarios() []*Scenario {
	var scenarios []*Scenario
	for gpuSeries := range DefaultGPUSeriesVMSizes {
		scenarios = append(scenarios, t.ubuntu2204gpu(gpuSeries))
	}
	return scenarios
}

func (t *Template) allScenarios() []*Scenario {
	return append(t.scenarios(), t.gpuVMSizeScenarios()...)
}

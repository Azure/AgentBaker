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
		// block for ubuntu 1804
		t.ubuntu1804(),
		t.ubuntu1804gpu(),
		t.ubuntu1804_azurecni(),
		t.ubuntu1804gpu_azurecni(),
		t.ubuntu1804SystemdChronyDropin(),
		t.ubuntu1804ChronyRestarts(),

		// block for ubuntu 2204
		t.ubuntu2204(),
		t.ubuntu2204ARM64(),
		t.ubuntu2204CustomSysctls(),
		t.ubuntu2204Wasm(),
		t.ubuntu2204gpuNoDriver(),
		t.ubuntu2204CustomCATrust(),
		t.ubuntu2204ArtifactStreaming(),
		t.ubuntu2204privatekubepkg(),
		t.ubuntu2204AirGap(),
		t.ubuntu2204ContainerdURL(),
		t.ubuntu2204ContainerdVersion(),
		t.ubuntu2204SystemdChronyDropin(),
		t.ubuntu2204ChronyRestarts(),

		// block for mariner v2
		t.marinerv2(),
		t.marinerv2ARM64(),
		t.marinerv2gpu(),
		t.marinerv2CustomSysctls(),
		t.marinerv2Wasm(),
		t.marinerv2_azurecni(),
		t.marinerv2gpu_azurecni(),
		t.marinerv2AirGap(),
		t.marinerv2ARM64AirGap(),
		t.marinerv2SystemdChronyDropin(),
		t.marinerv2ChronyRestarts(),

		// block for azurelinux v2
		t.azurelinuxv2(),
		t.azurelinuxv2ARM64(),
		t.azurelinuxv2gpu(),
		t.azurelinuxv2CustomSysctls(),
		t.azurelinuxv2Wasm(),
		t.azurelinuxv2_azurecni(),
		t.azurelinuxv2gpu_azurecni(),
		t.azurelinuxv2ARM64AirGap(),
		t.azurelinuxv2AirGap(),
		t.azurelinuxv2SystemdChronyDropin(),
		t.azurelinuxv2ChronyRestarts(),
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

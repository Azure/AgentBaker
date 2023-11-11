package scenario

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Azure/agentbakere2e/artifact"
	"github.com/Azure/agentbakere2e/suite"
)

// GetScenarios returns the set of scenarios comprising the E2E suite in tabular form.
func GetScenariosForSuite(ctx context.Context, suiteConfig *suite.Config) (Table, error) {
	var (
		table   = Table{}
		catalog = DefaultVHDCatalog
	)

	if suiteConfig.UseVHDsFromBuild() {
		log.Printf("will use VHDs from specified build: %d", suiteConfig.VHDBuildID)

		downloader, err := artifact.NewDownloader(ctx, suiteConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to construct new ADO artifact downloader: %w", err)
		}

		err = downloader.DownloadVHDBuildPublishingInfo(ctx, artifact.PublishingInfoDownloadOpts{
			BuildID:   suiteConfig.VHDBuildID,
			TargetDir: artifact.DefaultPublishingInfoDir,
			SKUList: []string{
				"1804-gen2-containerd",
				"2204-arm64-gen2-containerd",
				"2204-gen2-containerd",
				"azurelinuxv2-gen2-arm64",
				"azurelinuxv2-gen2",
				"marinerv2-gen2",
				"marinerv2-gen2-arm64",
			},
		})
		defer os.RemoveAll(artifact.DefaultPublishingInfoDir)
		if err != nil {
			return nil, fmt.Errorf("unable to download VHD publishing info: %w", err)
		}

		if err = catalog.addEntriesFromPublishingInfos(artifact.DefaultPublishingInfoDir); err != nil {
			return nil, fmt.Errorf("unable to load VHD selections from publishing info dir %s: %w", artifact.DefaultPublishingInfoDir, err)
		}
	}

	t := &Template{
		VHDCatalog: catalog,
	}

	for _, scenario := range scenarios(t) {
		if suiteConfig.ScenariosToRun != nil {
			if !suiteConfig.ScenariosToRun[scenario.Name] {
				continue
			}
		} else if suiteConfig.ScenariosToExclude != nil {
			if suiteConfig.ScenariosToExclude[scenario.Name] {
				continue
			}
		}
		log.Printf("will run E2E scenario %q: %s; with VHD: %s", scenario.Name, scenario.Description, scenario.VHDResourceID.Short())
		table[scenario.Name] = scenario
	}

	return table, nil
}

// This function is called internally by the scenario package to get each e2e scenario's respective config as one long slice.
// To add a sceneario, implement a new method on the Template type in a separate file that returns a *Scenario and add
// its return value to the slice returned by this function.
func scenarios(t *Template) []*Scenario {
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
	}
}

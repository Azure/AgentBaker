package runner

import (
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/scenario"
)

func validateVMSeriesOptions(c *config.Configuration, parallel int) error {
	if c.VMSeriesOS != "linux" && c.VMSeriesOS != "windows" {
		return fmt.Errorf("--vm-series-os must be linux or windows")
	}
	if parallel != 1 {
		return fmt.Errorf("VM-series coverage requires --parallel=1 to bound target-family quota usage")
	}
	if c.KeepVMSS {
		return fmt.Errorf("VM-series coverage cannot use --keep-vmss: test VMs must be deleted before the next scenario")
	}
	if strings.TrimSpace(c.SystemPoolVMSKU) == "" {
		return fmt.Errorf("VM-series coverage requires a non-empty --system-pool-vm-sku")
	}
	return nil
}

func partitionVMSeries(scenarios []*scenario.Scenario, architecture, osType string) ([]*scenario.Scenario, []scenarioResult, error) {
	var runnable []*scenario.Scenario
	var filtered []scenarioResult
	for _, s := range scenarios {
		tags := s.EffectiveTags()
		reason := ""
		switch {
		case !tags.VMSeriesCoverageTest && !tags.KernelCoverageTest:
			reason = "not a representative VM-series scenario"
		case tags.Arch != architecture:
			reason = "image architecture differs from requested VM SKU"
		case (tags.OS == string(config.OSWindows)) != (osType == "windows"):
			reason = "image is outside the orchestration build's OS cohort"
		case s.VHD == nil || s.VHD.UnsupportedGen2:
			reason = "VM-series coverage requires a Gen2 image"
		}
		if reason == "" {
			runnable = append(runnable, s)
			continue
		}
		filtered = append(filtered, scenarioResult{
			Name: s.Name, Status: statusSkipped,
			Attempts: []attemptResult{{Attempt: 1, Status: statusSkipped, Message: "filtered: " + reason}},
		})
	}
	if len(runnable) == 0 {
		return nil, filtered, fmt.Errorf("no compatible VM-series scenarios for architecture %q and OS %q (Windows ARM64 is unsupported)", architecture, osType)
	}
	return runnable, filtered, nil
}

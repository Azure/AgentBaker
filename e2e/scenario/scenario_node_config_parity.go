package scenario

import (
	"context"
	"slices"

	"github.com/Azure/agentbaker/e2e/config"
)

func NBCANCParityScenarioNames() []string {
	return []string{
		"Ubuntu2204_NodeHardening_KubeReservedSlice_CLIFlags_NBCANCParity",
		"Ubuntu2604Minimal_NodeHardening_KubeReservedSlice_CLIFlags_NBCANCParity",
		"Ubuntu2404_TemporaryKubeletDisk_NBCANCParity",
		"AzureLinuxV3_TemporaryKubeletDisk_NBCANCParity",
		"Ubuntu2604Minimal_CustomNodeConfig_NBCANCParity",
		"AzureLinuxV3Gen2Kata_NBCANCParity",
	}
}

func init() {
	for _, original := range List() {
		if !slices.Contains(NBCANCParityScenarioNames(), original.Name+"_NBCANCParity") {
			continue
		}

		s := *original
		s.Name += "_NBCANCParity"
		s.Description = "Checks NBC/ANC environment parity on the PR-built VHD; " + original.Description
		s.ForceScriptlessNBCCSECmd = true
		s.ExcludeFromNBCSampling = true
		s.SkipIf = func(ctx context.Context) string {
			if config.Config.VHDMetadataFile == "" {
				return "NBC/ANC parity requires VHD metadata from the PR's VHD build"
			}
			if config.Config.DisableScriptless || config.Config.TestPreProvision {
				return "NBC/ANC parity requires scriptless node provisioning"
			}
			if original.SkipIf != nil {
				return original.SkipIf(ctx)
			}
			return ""
		}
		Register(&s)
	}
}

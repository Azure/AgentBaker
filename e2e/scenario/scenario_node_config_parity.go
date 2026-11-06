package scenario

import (
	"context"

	"github.com/Azure/agentbaker/e2e/config"
)

func init() {
	for _, original := range List() {
		switch original.Name {
		case "Ubuntu2204_NodeHardening_KubeReservedSlice_CLIFlags",
			"Ubuntu2604Minimal_NodeHardening_KubeReservedSlice_CLIFlags",
			"Ubuntu2404_TemporaryKubeletDisk",
			"AzureLinuxV3_TemporaryKubeletDisk",
			"Ubuntu2604Minimal_CustomNodeConfig",
			"AzureLinuxV3Gen2Kata":
		default:
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

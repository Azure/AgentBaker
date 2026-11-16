package scenario

import (
	"context"
	"os"
	"strings"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// Each capture needs its own fresh node; ftrace repeats reveal run-to-run patterns.
func init() {
	for _, trial := range []struct {
		backend string
		name    string
	}{
		{backend: "ftrace", name: "Ubuntu2604Minimal_BootTrace_ftrace"},
		{backend: "ftrace", name: "Ubuntu2604Minimal_BootTrace_ftrace_2"},
		{backend: "ftrace", name: "Ubuntu2604Minimal_BootTrace_ftrace_3"},
		{backend: "lttng", name: "Ubuntu2604Minimal_BootTrace_lttng"},
	} {
		image := *config.VHDUbuntu2604MinimalGen2Containerd
		image.Name += "-boot-trace-" + trial.backend
		caExperiment := strings.EqualFold(os.Getenv("E2E_BOOT_TRACE_CA_EXPERIMENT"), "true")
		skipCAUpdate := caExperiment && trial.name == "Ubuntu2604Minimal_BootTrace_ftrace_3"
		var experimentFiles []CustomDataWriteFile
		if caExperiment {
			mode := "enabled"
			if skipCAUpdate {
				mode = "skip-update-ca-certificates"
			}
			experimentFiles = []CustomDataWriteFile{{
				Path: "/etc/aks-node-boot-trace-ca-update-mode", Permissions: "0600",
				Owner: "root:root", Content: mode + "\n",
			}}
		}
		Register(&Scenario{
			Name:        trial.name,
			Description: "Captures first-boot activity on a fresh node before workload validation",
			SkipIf: func(context.Context) string {
				if os.Getenv("E2E_BOOT_TRACE") != "true" {
					return "experimental boot tracing requires E2E_BOOT_TRACE=true"
				}
				if config.Config.TestPreProvision || config.Config.VHDMetadataFile == "" {
					return "boot tracing requires direct provisioning and exact VHD metadata"
				}
				return ""
			},
			Config: Config{
				Cluster:                ClusterLatestKubernetesVersionKubenet,
				VHD:                    &image,
				BootTraceBackend:       trial.backend,
				BootTraceSkipCAUpdate:  skipCAUpdate,
				CustomDataWriteFiles:   experimentFiles,
				ExcludeFromNBCSampling: true,
				// Match the VHD's binary cache without changing the shared cluster.
				BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.36.5"
				},
				AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
					cfg.KubernetesVersion = "1.36.5"
				},
			},
		})
	}
}

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
	if directory := os.Getenv("E2E_BOOT_TRACE_MATRIX_DIR"); directory != "" {
		if err := registerBootTraceMatrix(directory); err != nil {
			panic(err)
		}
		return
	}
	for _, name := range []string{
		"Ubuntu2604Minimal_BootTrace_ftrace",
		"Ubuntu2604Minimal_BootTrace_ftrace_2",
		"Ubuntu2604Minimal_BootTrace_ftrace_3",
	} {
		image := *config.VHDUbuntu2604MinimalGen2Containerd
		image.Name += "-boot-trace-ftrace"
		caExperiment := strings.EqualFold(os.Getenv("E2E_BOOT_TRACE_CA_EXPERIMENT"), "true")
		skipCAUpdate := caExperiment && name == "Ubuntu2604Minimal_BootTrace_ftrace_3"
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
			Name:        name,
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
				BootTraceBackend:       "ftrace",
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

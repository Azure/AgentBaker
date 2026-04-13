package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

// CSE performance thresholds for the golden image (cached) path.
// These represent the expected normal performance when all binaries are pre-cached on the VHD.
// If any of these are exceeded, it indicates a regression in CSE task ordering or apt lock contention.
// Thresholds are set at ~2-3x observed healthy values to allow for infrastructure variance
// while still catching real regressions (e.g., apt lock contention adds 30-50s).
var cachedCSEThresholds = CSETimingThresholds{
	TotalCSEThreshold: 35 * time.Second,
	TaskThresholds: map[string]time.Duration{
		"installDebPackageFromFile":  10 * time.Second, // healthy: ~2-4s with dpkg -i
		"aptmarkWALinuxAgent":         5 * time.Second, // healthy: ~0.5s with dpkg --set-selections
		"configureKubeletAndKubectl": 15 * time.Second, // healthy: ~5-8s
		"ensureContainerd":            8 * time.Second, // healthy: ~2-3s on cached path
	},
}

// CSE performance thresholds for the full install path.
// These are more generous since the full path includes downloading and installing packages.
var fullInstallCSEThresholds = CSETimingThresholds{
	TotalCSEThreshold: 90 * time.Second,
	TaskThresholds: map[string]time.Duration{
		"installDeps":                60 * time.Second, // healthy: ~20-30s
		"installContainerRuntime":    45 * time.Second, // healthy: ~15-25s
		"installDebPackageFromFile":  15 * time.Second, // healthy: ~5-8s
		"aptmarkWALinuxAgent":         8 * time.Second, // healthy: ~1-3s
		"configureKubeletAndKubectl": 25 * time.Second, // healthy: ~8-12s
		"ensureContainerd":           15 * time.Second, // healthy: ~5-8s
	},
}

func Test_Ubuntu2204_CSE_CachedPerformance(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Validates CSE timing on the golden image (cached) path where binaries are pre-installed on VHD. " +
			"Forces the PMC deb package install path (installKubeletKubectlFromPkg → installDebPackageFromFile) " +
			"by clearing CustomKubeBinaryURL and setting ShouldEnforceKubePMCInstall with k8s 1.34. " +
			"This catches regressions like apt lock contention when task ordering changes.",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Use k8s 1.34.4 because that's what has cached deb packages on the VHD.
				// The default 1.30 only has tarballs, not .deb files, so it would never
				// exercise the installDebPackageFromFile code path.
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.34.4"
				nbc.AgentPoolProfile.KubernetesConfig.CustomKubeProxyImage = "mcr.microsoft.com/oss/kubernetes/kube-proxy:v1.34.4"
				// Clear CustomKubeBinaryURL to prevent the URL-based install path.
				// In production, many nodes use the PMC deb package path, not the URL path.
				nbc.AgentPoolProfile.KubernetesConfig.CustomKubeBinaryURL = ""
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				// Force the PMC deb package install path even on the E2E cluster.
				// Without this, the CSE would fall back to the URL path which doesn't exercise
				// installDebPackageFromFile (the function that caused the regression).
				vmss.Tags["ShouldEnforceKubePMCInstall"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateCSETimings(ctx, s, cachedCSEThresholds)
			},
		},
	})
}

func Test_Ubuntu2204_CSE_FullInstallPerformance(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Validates CSE timing on the full install path where all dependencies are installed from scratch. " +
			"Uses SkipBinaryCleanup VMSS tag to force FULL_INSTALL_REQUIRED=true.",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["SkipBinaryCleanup"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateCSETimings(ctx, s, fullInstallCSEThresholds)
			},
		},
	})
}

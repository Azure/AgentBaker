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
//
// Thresholds are derived from production telemetry (Ubuntu 22.04, GuestAgentGenericLogs table,
// FA database on azcore cluster, ~35K samples per task over 30 minutes):
//   - Specific thresholds: set at ~p95 to catch regressions while tolerating normal infra variance
//   - DefaultTaskThreshold: catches any task >1s not covered by specific thresholds
//   - aptmarkWALinuxAgent: bimodal distribution (p50=0.49s, p99=58s) due to apt lock contention,
//     threshold at p90 since cached path should avoid lock contention
var cachedCSEThresholds = CSETimingThresholds{
	TotalCSEThreshold:    60 * time.Second,
	DefaultTaskThreshold: 45 * time.Second, // generous catch-all for untracked tasks >1s
	TaskThresholds: map[string]time.Duration{
		// Core kubelet/containerd install
		"installDebPackageFromFile":  22 * time.Second, // prod p50=3.88s p95=21.55s p99=42.88s
		"aptmarkWALinuxAgent":        24 * time.Second, // prod p50=0.49s p90=23.32s p95=37.47s (bimodal: apt lock)
		"configureKubeletAndKubectl": 27 * time.Second, // prod p50=6.56s p95=26.06s p99=44.39s
		"ensureContainerd":            3 * time.Second, // prod p50=0.94s p95=1.99s  p99=2.80s
		"ensureKubelet":              10 * time.Second, // prod p50=3.27s p95=6.20s  p99=10.01s
		"installContainerRuntime":     2 * time.Second, // prod p50=0.26s p95=0.50s  p99=0.85s
		"installStandaloneContainerd": 2 * time.Second, // prod p50=0.10s p95=0.18s  p99=0.46s

		// Kubelet install variants (only one fires per VM depending on install path)
		"installKubeletKubectlFromPkg": 38 * time.Second, // prod p50=14.68s p95=37.45s p99=56.59s (PMC deb path)
		"installKubeletKubectlFromURL": 10 * time.Second, // prod p50=5.43s  p95=9.59s  p99=15.65s (URL path)
		"extractKubeBinaries":          10 * time.Second, // prod p50=5.97s  p95=9.72s  p99=15.21s

		// Credential provider
		"installCredentialProviderFromUrl": 2 * time.Second,  // prod p50=1.01s p95=1.83s p99=2.89s
		"installCredentialProviderFromPkg": 5 * time.Second,  // prod p50=1.95s p95=4.72s p99=6.34s
		"downloadCredentialProvider":       2 * time.Second,  // prod p50=0.63s p95=1.27s p99=2.12s
		"installCredentialProvider":        3 * time.Second,  // prod p50=0.94s p95=2.68s p99=6.02s

		// Networking and node configuration
		"retrycmd_nslookup":      4 * time.Second, // prod p50=0.55s p95=3.89s p99=5.60s
		"configureNodeExporter": 44 * time.Second, // prod p50=1.62s p95=43.9s p99=117.45s (high tail!)
		"ensureSnapshotUpdate":   2 * time.Second, // prod p50=0.59s p95=1.15s p99=1.55s
	},
}

// CSE performance thresholds for the full install path.
// These are more generous since the full path includes downloading and installing packages.
//
// Thresholds are derived from production telemetry (Ubuntu 22.04, same source as cached).
// Full install thresholds are set at ~p99 since the full install path is rarer and more variable.
var fullInstallCSEThresholds = CSETimingThresholds{
	TotalCSEThreshold:    120 * time.Second,
	DefaultTaskThreshold: 60 * time.Second, // generous catch-all for untracked tasks
	TaskThresholds: map[string]time.Duration{
		// Core kubelet/containerd install
		"installDeps":                90 * time.Second, // no direct prod data; generous for full install
		"installContainerRuntime":    60 * time.Second, // prod p50=0.26s p99=0.78s (cached); much higher on full
		"installDebPackageFromFile":  45 * time.Second, // prod p99=42.88s
		"aptmarkWALinuxAgent":        60 * time.Second, // prod p99=58.07s (bimodal: apt lock contention)
		"configureKubeletAndKubectl": 45 * time.Second, // prod p99=44.39s
		"ensureContainerd":            5 * time.Second, // prod p99=2.80s; slightly higher for full install
		"ensureKubelet":              15 * time.Second, // prod p99=10.01s; slightly higher for full install
		"installStandaloneContainerd": 2 * time.Second, // prod p99=0.46s

		// Kubelet install variants
		"installKubeletKubectlFromPkg": 57 * time.Second, // prod p99=56.59s
		"installKubeletKubectlFromURL": 16 * time.Second, // prod p99=15.65s
		"extractKubeBinaries":          16 * time.Second, // prod p50=5.97s p95=9.72s p99=15.21s

		// Credential provider
		"installCredentialProviderFromUrl": 3 * time.Second,  // prod p99=2.89s
		"installCredentialProviderFromPkg": 7 * time.Second,  // prod p99=6.34s
		"installCredentialProviderFromPMC": 40 * time.Second, // prod p50=3.38s p95=23.66s p99=39.37s
		"downloadCredentialProvider":       3 * time.Second,  // prod p99=2.12s
		"installCredentialProvider":        7 * time.Second,  // prod p99=6.02s

		// Networking and node configuration
		"retrycmd_nslookup":      6 * time.Second,  // prod p99=5.60s
		"configureNodeExporter": 120 * time.Second, // prod p99=117.45s (extreme tail!)
		"ensureSnapshotUpdate":    2 * time.Second, // prod p99=1.55s
		"downloadPkgFromVersion":  4 * time.Second, // prod p50=0.30s p95=1.04s p99=3.39s
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

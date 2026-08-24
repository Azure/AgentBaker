package e2e

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"github.com/Masterminds/semver/v3"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

func EmptyBootstrapConfigMutator(_ *Cluster, configuration *datamodel.NodeBootstrappingConfiguration) {
}
func EmptyVMConfigMutator(vmss *armcompute.VirtualMachineScaleSet) {}

func DualStackConfigMutator(_ *Cluster, configuration *datamodel.NodeBootstrappingConfiguration) {
	properties := configuration.ContainerService.Properties
	properties.FeatureFlags.EnableIPv6DualStack = true
}

func Windows2025BootstrapConfigMutator(configuration *datamodel.NodeBootstrappingConfiguration) error {
	// 2025 supported in 1.32+ - a kubelet bug impacts networking in most of 1.32 and 1.33.0, .1
	version := components.GetKubeletVersionByMinorVersion("v1.33")
	if err := assert.NotEqual(version, "", "find a Windows 2025 kubelet version for Kubernetes 1.33"); err != nil {
		return err
	}
	configuration.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = components.RemoveLeadingV(version)
	return nil
}

func DualStackVMConfigMutator(set *armcompute.VirtualMachineScaleSet) {
	ip4Config := set.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations[0].Properties.IPConfigurations[0]

	ip6Config := &armcompute.VirtualMachineScaleSetIPConfiguration{
		Name: to.Ptr(fmt.Sprintf("%s_1", *ip4Config.Name)),
		Properties: &armcompute.VirtualMachineScaleSetIPConfigurationProperties{
			Primary:                 to.Ptr(false),
			PrivateIPAddressVersion: to.Ptr(armcompute.IPVersionIPv6),
			Subnet: &armcompute.APIEntityReference{
				ID: ip4Config.Properties.Subnet.ID,
			},
		},
	}

	set.Properties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations[0].Properties.IPConfigurations = []*armcompute.VirtualMachineScaleSetIPConfiguration{
		ip4Config,
		ip6Config,
	}
}

func Test_Windows2022_AzureNetwork(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 Azure Network",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2022Containerd,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "21H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2022AzureOverlayNetworkDualStack(t *testing.T) {
	t.Skip("Dual stack tests are not working yet")
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 Azure Overlay Network Dual Stack",
		Config: Config{
			Cluster:                ClusterAzureOverlayNetworkDualStack,
			VHD:                    config.VHDWindows2022Containerd,
			VMConfigMutator:        DualStackVMConfigMutator,
			BootstrapConfigMutator: DualStackConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "21H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2022Gen2AzureNetwork(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Azure Network - hyperv gen2",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2022ContainerdGen2,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd-gen2"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "21H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-current.zip"),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2022Gen2AzureOverlayNetworkDualStack(t *testing.T) {
	t.Skip("Dual stack tests are not working yet")
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Azure Overlay Network Dual Stack - hyperv gen 2",
		Config: Config{
			Cluster:                ClusterAzureOverlayNetworkDualStack,
			VHD:                    config.VHDWindows2022ContainerdGen2,
			VMConfigMutator:        DualStackVMConfigMutator,
			BootstrapConfigMutator: DualStackConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd-gen2"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "21H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "CSEScriptsPackageUrl used for provision is https://packages.aks.azure.com/aks/windows/cse/aks-windows-cse-scripts-current.zip"),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2025(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 with Containerd",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2025,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutatorWithError: func(_ context.Context, _ *Cluster, configuration *datamodel.NodeBootstrappingConfiguration) error {
				return Windows2025BootstrapConfigMutator(configuration)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "24H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2025Gen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 with Containerd - hyperv gen 2",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2025Gen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutatorWithError: func(_ context.Context, _ *Cluster, configuration *datamodel.NodeBootstrappingConfiguration) error {
				return Windows2025BootstrapConfigMutator(configuration)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025-gen2"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "24H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2025Gen2TrustedLaunch(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 Gen2 Trusted Launch (Secure Boot + vTPM)",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDWindows2025Gen2TL,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
			},
			BootstrapConfigMutatorWithError: func(_ context.Context, _ *Cluster, configuration *datamodel.NodeBootstrappingConfiguration) error {
				return Windows2025BootstrapConfigMutator(configuration)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025-gen2-tl"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "24H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2025Gen2_WindowsCiliumNetworking(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2025 Gen2 with Windows Cilium Networking (WCN) enabled",
		Config: Config{
			Cluster:               ClusterAzureNetwork,
			VHD:                   config.VHDWindows2025Gen2,
			VMConfigMutator:       EmptyVMConfigMutator,
			WaitForSSHAfterReboot: 5 * time.Minute,
			BootstrapConfigMutatorWithError: func(_ context.Context, _ *Cluster, configuration *datamodel.NodeBootstrappingConfiguration) error {
				if err := Windows2025BootstrapConfigMutator(configuration); err != nil {
					return err
				}
				if configuration.AgentPoolProfile.AgentPoolWindowsProfile == nil {
					configuration.AgentPoolProfile.AgentPoolWindowsProfile = &datamodel.AgentPoolWindowsProfile{}
				}
				configuration.AgentPoolProfile.AgentPoolWindowsProfile.NextGenNetworkingEnabled = to.Ptr(true)
				configuration.AgentPoolProfile.AgentPoolWindowsProfile.NextGenNetworkingConfig = to.Ptr("")
				return nil
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025-gen2"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "24H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateWindowsCiliumIsRunning(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2022_SecureTLSBootstrapping_BootstrapToken_Fallback(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd 2- hyperv gen 2 using secure TLS bootstrapping bootstrap token fallback",
		Tags: Tags{
			BootstrapTokenFallback: true,
		},
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2022ContainerdGen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{
					Enabled:                true,
					GetAccessTokenTimeout:  (10 * time.Second).String(),
					UserAssignedIdentityID: "invalid", // use an unexpected user-assigned identity ID to force a secure TLS bootstrapping failure
				}
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd-gen2"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "21H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2022_DisableKubeletServingCertificateRotationWithTags(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd 2- hyperv gen 2 with kubelet serving certificate rotation disabled by VMSS tag",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2022ContainerdGen2,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd-gen2"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "21H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2022_VHDCaching(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "VHD Caching",
		Config: Config{
			Cluster:    ClusterAzureNetwork,
			VHD:        config.VHDWindows2022Containerd, // gen1 is default for windows 2022
			VHDCaching: true,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				// If the VHD has incorrect settings (like network misconfiguration)
				// deploying more than one VM may expose the issue.
				// This check is not always reliable, since only one VM is created per test run in the current framework.
				// Therefore, tests may incorrectly pass more often than they fail in these cases.
				vmss.SKU.Capacity = to.Ptr[int64](2)
			},
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "21H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2025Gen2_VHDCaching(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "VHD Caching - Windows Server 2025 Gen2",
		Config: Config{
			Cluster:    ClusterAzureNetwork,
			VHD:        config.VHDWindows2025Gen2,
			VHDCaching: true,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Capacity = to.Ptr[int64](2)
			},
			BootstrapConfigMutatorWithError: func(_ context.Context, _ *Cluster, configuration *datamodel.NodeBootstrappingConfiguration) error {
				return Windows2025BootstrapConfigMutator(configuration)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025-gen2"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "24H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

// Test_Windows2022_VHDCaching_LegacyTLSBootstrap exercises Windows PIS /
// VHD-cached provisioning with secure TLS bootstrap disabled, forcing kubelet
// to use the legacy bootstrap-token path. Catches regressions in the two-stage
// CSE flow that only surface when no secure-tls-bootstrap client is around to
// overwrite the temporary kubeconfig.
//
// It also positively guards the BasePrep->NodePrep kubeconfig fix: a stale
// sentinel bootstrap token is baked during the pre-provision (BasePrep) stage,
// while the real cluster token is used at provision time. If bootstrap-config
// were written in BasePrep (the buggy behaviour), the cached VHD would carry the
// stale token and the node would fail to register; because it is written in
// NodePrep, the live token wins and the sentinel must never reach the node.
func Test_Windows2022_VHDCaching_LegacyTLSBootstrap(t *testing.T) {
	// Deliberately bogus but correctly-formatted ([a-z0-9]{6}.[a-z0-9]{16}) token.
	// Baked into the VHD at BasePrep time only; must be overwritten by the live
	// token in NodePrep. The bake stage is PreProvisionOnly (no kubelet start), so
	// this bogus value never breaks stage 1.
	const staleBakeTimeToken = "baketk.000000000000bake"
	RunScenario(t, &Scenario{
		Description: "VHD Caching with secure TLS bootstrap disabled",
		Config: Config{
			Cluster:    ClusterAzureNetwork,
			VHD:        config.VHDWindows2022Containerd,
			VHDCaching: true,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Capacity = to.Ptr[int64](2)
			},
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				if nbc.SecureTLSBootstrappingConfig == nil {
					nbc.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{}
				}
				nbc.SecureTLSBootstrappingConfig.Enabled = false
			},
			// Bake stage only: inject the stale sentinel token so the provision-stage
			// validator can prove bootstrap-config is (re)written from the live token.
			PreProvisionBootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.KubeletClientTLSBootstrapToken = to.Ptr(staleBakeTimeToken)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					// The provisioned node must use the live token written in NodePrep,
					// never the stale token baked during VHD creation.
					ValidateFileHasContent(ctx, s, "C:\\k\\bootstrap-config", s.GetTLSBootstrapToken()),
					ValidateFileExcludesContent(ctx, s, "C:\\k\\bootstrap-config", staleBakeTimeToken),
				)
			},
		},
	})
}

func Test_Windows2022Gen2_k8s_133(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Windows Server 2022 with Containerd 2- hyperv gen 2",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2022ContainerdGen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(_ *Cluster, configuration *datamodel.NodeBootstrappingConfiguration) {
				// 2025 supported in 1.32+ .
				configuration.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.33.1"
				configuration.K8sComponents.WindowsPackageURL = fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/windowszip/v%s-1int.zip", "1.33.1", "1.33.1")
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2022-containerd-gen2"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2022 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "21H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}
func Test_Windows2022_McrChinaCloud_Windows(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			MockAzureChinaCloud: true,
		},
		Description: "Windows Server 2022 Azure Network Containerd - v1 to test Azure China Cloud MCR host",
		Config: Config{
			Cluster:                ClusterAzureNetwork,
			VHD:                    config.VHDWindows2022Containerd,
			VMConfigMutator:        EmptyVMConfigMutator,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateFileExists(ctx, s, `C:\ProgramData\containerd\certs.d\docker.io\hosts.toml`),
					ValidateFileExists(ctx, s, `C:\ProgramData\containerd\certs.d\mcr.azk8s.cn\hosts.toml`),
					ValidateFileHasContent(ctx, s,
						`C:\ProgramData\containerd\certs.d\docker.io\hosts.toml`,
						`https://docker.io`),
					ValidateFileHasContent(ctx, s,
						`C:\ProgramData\containerd\certs.d\mcr.azk8s.cn\hosts.toml`,
						`https://mcr.azk8s.cn`),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
					ValidateCollectWindowsLogsScript(ctx, s),
				)
			},
		},
	})
}

func Test_Windows2025Gen2_McrChinaCloud_Windows(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			MockAzureChinaCloud: true,
		},
		Description: "Windows Server 2025 with Containerd - hyperv gen 2 to test Azure China Cloud MCR host",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2025Gen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutatorWithError: func(_ context.Context, _ *Cluster, configuration *datamodel.NodeBootstrappingConfiguration) error {
				return Windows2025BootstrapConfigMutator(configuration)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateWindowsVersionFromWindowsSettings(ctx, s, "2025-gen2"),
					ValidateWindowsProductName(ctx, s, "Windows Server 2025 Datacenter"),
					ValidateWindowsDisplayVersion(ctx, s, "24H2"),
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					ValidateKubeletArgs(ctx, s),
					ValidateContainerdWindowsPriorityClass(ctx, s),
					ValidateCiliumIsNotRunningWindows(ctx, s),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateFileExists(ctx, s, `C:\ProgramData\containerd\certs.d\docker.io\hosts.toml`),
					ValidateFileExists(ctx, s, `C:\ProgramData\containerd\certs.d\mcr.azk8s.cn\hosts.toml`),
					ValidateFileHasContent(ctx, s,
						`C:\ProgramData\containerd\certs.d\docker.io\hosts.toml`,
						`https://docker.io`),
					ValidateFileHasContent(ctx, s,
						`C:\ProgramData\containerd\certs.d\mcr.azk8s.cn\hosts.toml`,
						`https://mcr.azk8s.cn`),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
				)
			},
		},
	})
}

func Test_NetworkIsolatedCluster_Windows_WithEgress(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that Windows nodes in network isolated clusters configure containerd to use the bootstrap profile container registry for MCR images",
		Tags: Tags{
			NetworkIsolated: true,
			NonAnonymousACR: true,
		},
		Config: Config{
			Cluster: ClusterAzureBootstrapProfileCache,
			VHD:     config.VHDWindows2025Gen2,
			BootstrapConfigMutatorWithError: func(_ context.Context, _ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) error {
				if err := Windows2025BootstrapConfigMutator(nbc); err != nil {
					return err
				}
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io/aks-managed-repository", config.PrivateACRNameNotAnon(config.Config.DefaultLocation)),
					},
				}
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.AgentPoolProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.KubeletConfig["--image-credential-provider-config"] = "c:\\k\\credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "c:\\var\\lib\\kubelet\\credential-provider"
				orchestratorVersion, _ := semver.NewVersion(nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				if orchestratorVersion.LessThan(semver.MustParse("1.32.0")) {
					nbc.K8sComponents.WindowsCredentialProviderURL = fmt.Sprintf(
						"https://packages.aks.azure.com/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-windows-amd64-v%s.tar.gz",
						nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion,
						nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				} else {
					nbc.K8sComponents.WindowsCredentialProviderURL = fmt.Sprintf(
						"https://packages.aks.azure.com/dalec-packages/azure-acr-credential-provider/%s/windows/amd64/azure-acr-credential-provider_%s-1_amd64.zip",
						nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion,
						nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				}
				return nil
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					// Verify mcr.microsoft.com host config exist
					ValidateFileExists(ctx, s, `C:\ProgramData\containerd\certs.d\mcr.microsoft.com\hosts.toml`),
					ValidateFileDoesNotExist(ctx, s, `C:\ProgramData\containerd\certs.d\mcr.azk8s.cn\hosts.toml`),
					ValidateDotnetNotInstalledWindows(ctx, s),
					ValidateWindowsSystemServicesRestartConfiguration(ctx, s),
				)
			},
		},
	})
}

func Test_NetworkIsolatedCluster_Windows_OrasDownload(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that Windows nodes in network isolated clusters download kubelet/containerd binaries via ORAS when BootstrapProfileContainerRegistryServer is set",
		Tags: Tags{
			NetworkIsolated: true,
			NonAnonymousACR: false,
		},
		Config: Config{
			Cluster:         ClusterAzureBootstrapProfileCache,
			VHD:             config.VHDWindows2025Gen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutatorWithError: func(_ context.Context, _ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) error {
				if err := Windows2025BootstrapConfigMutator(nbc); err != nil {
					return err
				}
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io/aks-managed-repository", config.PrivateACRName(config.Config.DefaultLocation)),
						TestMode:                true,
					},
				}
				return nil
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return errors.Join(
					ValidateFileHasContent(ctx, s, "/k/kubeletstart.ps1", "--container-runtime=remote"),
					// Verify kubelet binaries were downloaded via ORAS instead of HTTP
					ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "Start to download kubelet binaries with oras"),
					ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "Start to download containerd with oras"),
				)
			},
		},
	})
}

// REVERT ME: self-contained Windows branch-CSE builder. Builds a CSE zip from the branch's
// staging/cse/windows/ and uploads it to E2E blob storage with a collision-free name so
// concurrent branch runs never overwrite each other's zip. Kept local here so the shared
// RCV1P helpers stay untouched. Remove once the CSE scripts ship.
var (
	winBranchCSEZipURL  string
	winBranchCSEZipErr  error
	winBranchCSEZipOnce sync.Once
)

func getOrBuildWindowsBranchCSEPackageURL() (string, error) {
	winBranchCSEZipOnce.Do(func() {
		// 5m covers a cold-start storage account create plus the zip build/upload.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		// The mutator runs at scenario-init time, before RunScenario creates the per-sub blob
		// storage account, so ensure the RG and identity/storage exist first.
		if _, err := CachedEnsureResourceGroup(ctx, config.Config.DefaultLocation); err != nil {
			winBranchCSEZipErr = fmt.Errorf("ensure shared resource group: %w", err)
			return
		}
		if _, err := CachedCreateVMManagedIdentity(ctx, config.Config.DefaultLocation); err != nil {
			winBranchCSEZipErr = fmt.Errorf("ensure shared storage account: %w", err)
			return
		}
		winBranchCSEZipURL, winBranchCSEZipErr = buildAndUploadWindowsCSEZip(ctx)
	})
	if winBranchCSEZipErr != nil {
		return "", fmt.Errorf("build or upload branch CSE zip: %w", winBranchCSEZipErr)
	}
	return winBranchCSEZipURL, nil
}

func buildAndUploadWindowsCSEZip(ctx context.Context) (string, error) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return "", fmt.Errorf("find repo root: %w", err)
	}
	cseDir := filepath.Join(repoRoot, "staging", "cse", "windows")

	tmpFile, err := os.CreateTemp("", "aks-windows-cse-scripts-branch-*.zip")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)
	err = filepath.Walk(cseDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(cseDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		// skip test files and debug helper (matches windows_package_cse.sh)
		if strings.HasSuffix(rel, ".tests.ps1") || strings.Contains(rel, ".tests.suites") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "README" || rel == "debug/update-scripts.ps1" {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		w, err := zw.Create(rel)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", rel, err)
		}
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open %s: %w", path, err)
		}
		_, copyErr := io.Copy(w, f)
		closeErr := f.Close()
		if copyErr != nil {
			return fmt.Errorf("copy %s: %w", path, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close %s: %w", path, closeErr)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("build zip: %w", err)
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("close zip writer: %w", err)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek temp file: %w", err)
	}

	// Build/run ID plus a random suffix so concurrent branch runs never pick the same blob
	// name and overwrite each other's CSE zip (BuildID defaults to "local" off-CI).
	blobName := fmt.Sprintf("cse-packages/aks-windows-cse-scripts-branch-%s-%s-%s.zip",
		config.Config.BuildID,
		time.Now().UTC().Format("20060102-150405"),
		randomLowercaseString(6))
	url, err := uploadWindowsCSEZipNoOverwrite(ctx, blobName, tmpFile)
	if err != nil {
		return "", fmt.Errorf("upload CSE zip: %w", err)
	}
	return url, nil
}

// uploadWindowsCSEZipNoOverwrite uploads without overwriting an existing blob (fails on
// conflict) and returns a SAS-signed URL, leaving the shared config upload helpers untouched.
func uploadWindowsCSEZipNoOverwrite(ctx context.Context, blobName string, file *os.File) (string, error) {
	client := config.Azure.Blob
	_, err := client.UploadFile(ctx, config.Config.BlobContainer, blobName, file, &azblob.UploadFileOptions{
		AccessConditions: &blob.AccessConditions{
			ModifiedAccessConditions: &blob.ModifiedAccessConditions{
				IfNoneMatch: to.Ptr(azcore.ETagAny), // create only if the blob does not already exist
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("upload blob %q without overwrite: %w", blobName, err)
	}

	expiry := time.Now().Add(6 * time.Hour).UTC()
	udc, err := client.ServiceClient().GetUserDelegationCredential(ctx, service.KeyInfo{
		Expiry: to.Ptr(expiry.Format(sas.TimeFormat)),
		Start:  to.Ptr(time.Now().UTC().Format(sas.TimeFormat)),
	}, nil)
	if err != nil {
		return "", fmt.Errorf("get user delegation credential: %w", err)
	}

	sig, err := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		ExpiryTime:    expiry,
		Permissions:   to.Ptr(sas.BlobPermissions{Read: true}).String(),
		ContainerName: config.Config.BlobContainer,
		BlobName:      blobName,
	}.SignWithUserDelegation(udc)
	if err != nil {
		return "", fmt.Errorf("sign blob: %w", err)
	}

	return fmt.Sprintf("%s/%s/%s?%s", config.Config.BlobStorageAccountURL(), config.Config.BlobContainer, blobName, sig.Encode()), nil
}

func Test_Windows2022_Dalec_CredentialProvider(t *testing.T) {
	// REVERT ME: build & use a CSE zip from the branch's staging/cse/windows/ so the resolver
	// change is exercised instead of the published package. Remove once the CSE scripts ship.
	cseURL, err := getOrBuildWindowsBranchCSEPackageURL()
	if err != nil {
		t.Error(err)
		return
	}

	RunScenario(t, &Scenario{
		Description: "Tests Windows 2022 node installs credential provider from dalec via components.json for k8s >= 1.33",
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             config.VHDWindows2022ContainerdGen2,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.WindowsProfile.CseScriptsPackageURL = cseURL // Remove once the CSE scripts ship
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.33.1"
				nbc.K8sComponents.WindowsPackageURL = fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/windowszip/v%s-1int.zip", "1.33.1", "1.33.1")
				nbc.KubeletConfig["--image-credential-provider-config"] = "c:\\k\\credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "c:\\var\\lib\\kubelet\\credential-provider"
				// Do NOT set WindowsCredentialProviderURL — resolver should use components.json
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				// Assert dalec-specific CSE log line to prove resolver path was taken
				return errors.Join(
					ValidateFileExists(ctx, s, `c:\var\lib\kubelet\credential-provider\acr-credential-provider.exe`),
					ValidateFileHasContent(ctx, s, "/AzureData/CustomDataSetupScript.log", "Using dalec credential provider"),
				)
			},
		},
	})
}

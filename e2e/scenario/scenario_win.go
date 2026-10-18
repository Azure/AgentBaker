package scenario

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"time"

	"github.com/Azure/agentbaker/e2e/assert"
	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Masterminds/semver/v3"
	"github.com/tidwall/gjson"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
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

const sshKeyInterpolationComment = `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDIs9weXqhc498AY/775zoJAO+bsmgBx2/V2KTaQgbU1I9ePbquox6r1idf1hcyR+wo9bqlMErLlSGdDCZqTfRmZS9gBbicxPuaIDnIvzfNBH/4Eqq6YVcwjkFeHWqL4ABPq/VNzbLr7JkkCVw9Widh3K/HTsfaDx13qOUwzcm2F7FN/+zvrRyz9RDwkzdeOVhG6JwHdQAjLM40z49BP4yPyHl4rxvDmGUcOYRy+zCf4Sz75Nw+7wOph3X8KUY8EcHqptXMtk+6f17tasZNaiK0sGY+Hq/Craz2ryO3cDtDn+8Kw2Mpwox8qmdKTCVHPkHgh9OwiFPPWcnld4kNg/+V9ONlsJLUTAwOVezqsCWWU/+NpTWhKqLp682FOZ1fhI+jRlMp0Sa6uEXdw9U56J4IbgzXa1RXYmmq8xceMRIRWC9dxVjcv8F1KrpJoCORtrZDQDaF3Kw789dX09MawfdCZscKSVDXRqvV7TWO2hndliQq3BW385ZkiephlrmpUVM= $(hostname)-literal-test`

var _ = Register(&Scenario{
	Name:        "Windows2022_AzureNetwork",
	Description: "Windows Server 2022 Azure Network",
	Config: Config{
		Cluster:         ClusterAzureNetwork,
		VHD:             config.VHDWindows2022Containerd,
		VMConfigMutator: EmptyVMConfigMutator,
		BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
			nbc.ContainerService.Properties.LinuxProfile.SSH.PublicKeys = append(
				nbc.ContainerService.Properties.LinuxProfile.SSH.PublicKeys,
				datamodel.PublicKey{KeyData: sshKeyInterpolationComment},
			)
		},
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
				ValidateSSHKeyLiteralPreservation(ctx, s, sshKeyInterpolationComment),
			)
		},
	},
})

var _ = Register(&Scenario{
	Name:        "Windows2022AzureOverlayNetworkDualStack",
	SkipReason:  "Dual stack tests are not working yet",
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

var _ = Register(&Scenario{
	Name:        "Windows2022Gen2AzureNetwork",
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

var _ = Register(&Scenario{
	Name:        "Windows2022Gen2AzureOverlayNetworkDualStack",
	SkipReason:  "Dual stack tests are not working yet",
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

var _ = Register(&Scenario{
	Name:        "Windows2025",
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

var _ = Register(&Scenario{
	Name:        "Windows2025Gen2",
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

var _ = Register(&Scenario{
	Name:        "Windows2025Gen2TrustedLaunch",
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

var _ = Register(&Scenario{
	Name:        "Windows2025Gen2_WindowsCiliumNetworking",
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

var _ = Register(&Scenario{
	Name:        "Windows2022_SecureTLSBootstrapping_BootstrapToken_Fallback",
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

var _ = Register(&Scenario{
	Name:        "Windows2022_DisableKubeletServingCertificateRotationWithTags",
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

var _ = Register(&Scenario{
	Name:        "Windows2022_VHDCaching",
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

var _ = Register(&Scenario{
	Name:        "Windows2025Gen2_VHDCaching",
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

// Windows2022_VHDCaching_LegacyTLSBootstrap exercises Windows PIS /
// VHD-cached provisioning with secure TLS bootstrap disabled, forcing kubelet
// to use the legacy bootstrap-token path. The bake payload omits the token, so
// successful node registration and the bootstrap-config check prove NodePrep
// uses the live provision-time token.
var _ = Register(newWindows2022_VHDCaching_LegacyTLSBootstrapScenario())

const windowsPISBakeBootstrapToken = "pisbak.0000000000000000"

func newWindows2022_VHDCaching_LegacyTLSBootstrapScenario() *Scenario {
	return &Scenario{
		Name:        "Windows2022_VHDCaching_LegacyTLSBootstrap",
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
			PreProvisionBootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.KubeletClientTLSBootstrapToken = to.Ptr(windowsPISBakeBootstrapToken)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				return ValidateWindowsFileContainsBootstrapToken(ctx, s, "C:\\k\\bootstrap-config")
			},
		},
	}
}

var _ = Register(&Scenario{
	Name:        "Windows2022Gen2_k8s_133",
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
var _ = Register(&Scenario{
	Name: "Windows2022_McrChinaCloud_Windows",
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

var _ = Register(&Scenario{
	Name: "Windows2025Gen2_McrChinaCloud_Windows",
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

var _ = Register(&Scenario{
	Name:        "NetworkIsolatedCluster_Windows_WithEgress",
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

var _ = Register(&Scenario{
	Name:        "NetworkIsolatedCluster_Windows_OrasDownload",
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

// The Kubernetes minors on either side of the gate require separate nodes for each generation.
// Only this pipeline run's VHDs establish that branch CSE and components were baked in.
var _ = Register(newWindowsCredentialProviderScenario("Windows2022_Dalec_CredentialProvider", config.VHDWindows2022ContainerdGen2, 33))
var _ = Register(newWindowsCredentialProviderScenario("Windows2022Gen1_Dalec_CredentialProvider", config.VHDWindows2022Containerd, 33))
var _ = Register(newWindowsCredentialProviderScenario("Windows2022_Legacy_CredentialProvider_Kubernetes132", config.VHDWindows2022ContainerdGen2, 32))
var _ = Register(newWindowsCredentialProviderScenario("Windows2022Gen1_Legacy_CredentialProvider_Kubernetes132", config.VHDWindows2022Containerd, 32))

func newWindowsCredentialProviderScenario(name string, vhd *config.Image, minor uint64) *Scenario {
	description := "Tests Windows 2022 VHD-embedded CSE selects the Dalec credential provider at Kubernetes 1.33 with a stock RP legacy URL, without falling back"
	if minor == 32 {
		description = "Tests Windows 2022 VHD-embedded CSE installs the legacy credential provider at Kubernetes 1.32 below the Dalec gate"
	}
	return &Scenario{
		Name:        name,
		Description: description,
		SkipIf: func(context.Context) string {
			return skipWindowsCredentialProvider(vhd)
		},
		Config: Config{
			Cluster:         ClusterAzureNetwork,
			VHD:             vhd,
			VMConfigMutator: EmptyVMConfigMutator,
			BootstrapConfigMutatorWithError: func(ctx context.Context, cluster *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) error {
				return windowsCredentialProviderMutator(ctx, cluster, nbc, minor)
			},
			Validator: func(ctx context.Context, s *Scenario) error {
				if minor == 32 {
					return errors.Join(
						ValidateFileExists(ctx, s, `C:\var\lib\kubelet\credential-provider\acr-credential-provider.exe`),
						ValidateFileExists(ctx, s, `C:\k\credential-provider-config.yaml`),
						ValidateFileExcludesContent(ctx, s, `C:\AzureData\CustomDataSetupScript.log`, "Using dalec credential provider"),
						ValidateFileExcludesContent(ctx, s, `C:\AzureData\CustomDataSetupScript.log`, "Failed to install dalec credential provider, falling back to RP URL."),
						validateWindowsLegacyCredentialProvider(ctx, s),
					)
				}
				return errors.Join(
					ValidateFileExists(ctx, s, `C:\var\lib\kubelet\credential-provider\acr-credential-provider.exe`),
					ValidateFileExists(ctx, s, `C:\k\credential-provider-config.yaml`),
					ValidateFileHasContent(ctx, s, `C:\AzureData\CustomDataSetupScript.log`, "Using dalec credential provider"),
					ValidateFileExcludesContent(ctx, s, `C:\AzureData\CustomDataSetupScript.log`, "Failed to install dalec credential provider, falling back to RP URL."),
				)
			},
		},
	}
}

func skipWindowsCredentialProvider(vhd *config.Image) string {
	const reason = "Windows credential-provider scenarios require a VHD built with branch CSE in the current pipeline run"
	// Explicit versions and metadata take precedence over the gallery tag selector.
	if vhd.Version != "" || config.Config.VHDMetadataFile != "" {
		return reason + "; pinned versions and VHD metadata overrides do not establish that provenance"
	}
	// e2e_run.sh maps VHD_BUILD_ID to the selector, but it may name an older build.
	// BUILD_BUILDID is Azure DevOps' current run ID, shared by CSE packaging and VHD build.
	buildID := os.Getenv("BUILD_BUILDID")
	if buildID == "" || config.Config.SIGVersionTagName != "buildId" || config.Config.SIGVersionTagValue != buildID {
		return fmt.Sprintf("%s; require buildId=BUILD_BUILDID (current run %q), got %s=%q",
			reason, buildID, config.Config.SIGVersionTagName, config.Config.SIGVersionTagValue)
	}
	return ""
}

func windowsCredentialProviderMutator(ctx context.Context, _ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration, minor uint64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return configureWindowsCredentialProvider(nbc,
		components.GetExpectedPackageVersions("kubernetes-binaries", "windows", "default"),
		components.GetExpectedPackageVersions("windows credential provider", "windows", "default"), minor)
}

func configureWindowsCredentialProvider(nbc *datamodel.NodeBootstrappingConfiguration, kubeletVersions, legacyProviderVersions []string, minor uint64) error {
	version, err := credentialProviderVersion(kubeletVersions, "kubernetes-binaries/windows/default", minor)
	if err != nil {
		return err
	}
	// Legacy provider releases differ from Kubernetes patches; use their own
	// cached metadata to keep the populated RP fallback URL valid.
	legacyVersion, err := credentialProviderVersion(legacyProviderVersions, "windows credential provider/windows/default", minor)
	if err != nil {
		return err
	}
	legacyURL, err := windowsLegacyCredentialProviderURL(legacyVersion)
	if err != nil {
		return err
	}
	nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = version
	nbc.K8sComponents.WindowsPackageURL = fmt.Sprintf("https://packages.aks.azure.com/kubernetes/v%s/windowszip/v%s-1int.zip", version, version)
	nbc.K8sComponents.WindowsCredentialProviderURL = legacyURL
	nbc.KubeletConfig["--image-credential-provider-config"] = `C:\k\credential-provider-config.yaml`
	nbc.KubeletConfig["--image-credential-provider-bin-dir"] = `C:\var\lib\kubelet\credential-provider`
	return nil
}

func credentialProviderVersion(versions []string, metadataPath string, minor uint64) (string, error) {
	var latest *semver.Version
	for _, raw := range versions {
		version, err := semver.StrictNewVersion(components.RemoveLeadingV(raw))
		if err != nil || version.Major() != 1 || version.Minor() != minor || version.Prerelease() != "" {
			continue
		}
		if latest == nil || version.GreaterThan(latest) {
			latest = version
		}
	}
	if latest == nil {
		return "", fmt.Errorf("credential-provider scenario requires a cached 1.%d patch in components.json %s", minor, metadataPath)
	}
	return latest.String(), nil
}

func windowsLegacyCredentialProviderURL(version string) (string, error) {
	// Match the version helper's source-relative lookup, independent of the runner's CWD.
	_, filename, _, _ := runtime.Caller(0)
	metadataPath := filepath.Join(filepath.Dir(filename), "..", "..", "parts", "common", "components.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return "", fmt.Errorf("read credential-provider metadata: %w", err)
	}
	return resolveWindowsLegacyCredentialProviderURL(data, version)
}

func resolveWindowsLegacyCredentialProviderURL(data []byte, version string) (string, error) {
	const metadataPath = "components.json windows credential provider/windows/default/downloadURL"
	if !gjson.ValidBytes(data) {
		return "", fmt.Errorf("%s: invalid JSON", metadataPath)
	}
	template := gjson.GetBytes(data, `Packages.#(name="windows credential provider").downloadURIs.windows.default.downloadURL`)
	if template.Type != gjson.String || !strings.Contains(template.String(), "${version}") {
		return "", fmt.Errorf("%s: requires a downloadURL template containing ${version}", metadataPath)
	}
	resolved := strings.ReplaceAll(template.String(), "${version}", version)
	parsed, err := url.Parse(resolved)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || strings.ContainsAny(resolved, "$ \t\r\n") {
		return "", fmt.Errorf("%s: invalid resolved URL %q", metadataPath, resolved)
	}
	return resolved, nil
}

func validateWindowsLegacyCredentialProvider(ctx context.Context, s *Scenario) error {
	version, err := credentialProviderVersion(components.GetExpectedPackageVersions("windows credential provider", "windows", "default"), "windows credential provider/windows/default", 32)
	if err != nil {
		return err
	}
	legacyURL, err := windowsLegacyCredentialProviderURL(version)
	if err != nil {
		return err
	}
	log, err := getFileContent(ctx, s, `C:\AzureData\CustomDataSetupScript.log`)
	if err != nil {
		return err
	}
	return validateWindowsLegacyCredentialProviderLog(log, legacyURL)
}

func validateWindowsLegacyCredentialProviderLog(log, legacyURL string) error {
	parsed, err := url.Parse(legacyURL)
	if err != nil {
		return fmt.Errorf("parse legacy credential-provider URL: %w", err)
	}
	filename := path.Base(parsed.Path)
	for _, line := range strings.Split(log, "\n") {
		// DownloadFileOverHttp emits these only after selecting the legacy path.
		// Match the archive name because Update-BaseUrl may remap the download host.
		cached := strings.Contains(line, "Using cached version of "+filename+" - Copying file from ")
		downloaded := strings.Contains(line, "Downloaded file ") && strings.Contains(line, "/"+filename+" to ")
		if (cached || downloaded) && strings.Contains(line, `\credentialprovider.tar.gz`) {
			return nil
		}
	}
	return fmt.Errorf("expected legacy credential-provider cache-copy or completed-download log for %s to credentialprovider.tar.gz", filename)
}

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/components"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/require"
)

func Test_AzureLinux3OSGuard(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using an Azure Linux V3 OS Guard VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinux3OSGuard,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.LocalDNSProfile = nil
			},
			Validator: func(ctx context.Context, s *Scenario) {},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
			},
		},
	})
}

func Test_Flatcar(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a Flatcar VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDFlatcarGen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/protocols", "protocols definition file")
				ValidateFileIsRegularFile(ctx, s, "/etc/ssl/certs/ca-certificates.crt")
			},
		},
	})
}

func Test_Flatcar_CustomCATrust(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Flatcar VHD can be properly bootstrapped and custom CA was correctly added",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDFlatcarGen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
					CustomCATrustCerts: []string{
						encodedTestCert,
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/opt/certs")
				// openssl x509 -hash of input cert
				ValidateFileExists(ctx, s, "/etc/ssl/certs/5c3b39ed.0")
			},
		},
	})
}

func Test_Flatcar_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a Flatcar and the self-contained installer can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDFlatcarGen2,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
			},
		},
	})
}

func Test_Flatcar_ARM64(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a Flatcar VHD on ARM64 architecture can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDFlatcarGen2Arm64,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.IsARM64 = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Test_Flatcar_AzureCNI(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Flatcar scenario on a cluster configured with Azure CNI",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDFlatcarGen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
			},
		},
	})
}

func Test_Flatcar_AzureCNI_ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Test Flatcar scenario on a cluster configured with Azure CNI and the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDFlatcarGen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
			},
		},
	})
}

func Test_Flatcar_AzureCNI_ChronyRestarts_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Test Flatcar scenario on a cluster configured with Azure CNI and the chrony service restarts if it is killed",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDFlatcarGen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.NetworkConfig.NetworkPlugin = aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
			},
		},
	})
}

func Test_Flatcar_SecureTLSBootstrapping_BootstrapToken_Fallback(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a Flatcar Gen2 VHD can be properly bootstrapped even if secure TLS bootstrapping fails",
		Tags: Tags{
			BootstrapTokenFallback: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDFlatcarGen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{
					Enabled:                true,
					Deadline:               (10 * time.Second).String(),
					UserAssignedIdentityID: "invalid", // use an unexpected user-assigned identity ID to force a secure TLS bootstrapping failure
				}
			},
		},
	})
}

func Test_AzureLinuxV3_SecureTLSBootstrapping_BootstrapToken_Fallback(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV3 Gen2 VHD can be properly bootstrapped even if secure TLS bootstrapping fails",
		Tags: Tags{
			BootstrapTokenFallback: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{
					Enabled:                true,
					Deadline:               (10 * time.Second).String(),
					UserAssignedIdentityID: "invalid", // use an unexpected user-assigned identity ID to force a secure TLS bootstrapping failure
				}
			},
		},
	})
}

func Test_AzureLinuxV3_AzureCNI(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "azurelinuxv3 scenario on a cluster configured with Azure CNI",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
			},
		},
	})
}

func Test_AzureLinuxV3_ChronyRestarts(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
			},
		},
	})
}

// Returns config for the 'base' E2E scenario

func Test_Ubuntu2204_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using self contained installer can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
			},
		},
	})
}

func Test_Ubuntu2204_Failure_Scriptless(t *testing.T) {
	err := RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using self contained installer can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileExists(ctx, s, "/opt/azure/containers/provision.complete")
				ValidateFileExists(ctx, s, "/var/log/azure/aks/provision.json")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				// Intentionally causing a failure here
				//config.Version = "v200"
				config.BootstrappingConfig = nil
				config.KubernetesCaCert = ""
			},
			ReturnErrorOnVMSSCreation: true,
		},
	})

	// Expect the error to contain API server connection failure since we provided invalid config
	require.ErrorContains(t, err, "API server connection check code: 51")
}

func Test_Ubuntu2204_Early_Failure_Scriptless(t *testing.T) {
	err := RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using self contained installer can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileExists(ctx, s, "/opt/azure/containers/provision.complete")
				ValidateFileExists(ctx, s, "/var/log/azure/aks/provision.json")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				// Intentionally causing a failure here
				config.Version = "VeryBadVersion"
			},
			ReturnErrorOnVMSSCreation: true,
		},
	})

	// Expect the error to contain unsupported version
	require.ErrorContains(t, err, "unsupported version: VeryBadVersion")
}

func Test_Ubuntu2404_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "testing that a new ubuntu 2404 node using self contained installer can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
			},
		},
	})
}

// Returns config for the 'gpu' E2E scenario
func Test_Ubuntu2204(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Check that we don't leak these secrets if they're
				// set (which they mostly aren't in these scenarios).
				nbc.ContainerService.Properties.CertificateProfile.ClientPrivateKey = "client cert private key"
				nbc.ContainerService.Properties.ServicePrincipalProfile.Secret = "SP secret"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", components.GetExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
				ValidateInstalledPackageVersion(ctx, s, "moby-runc", components.GetExpectedPackageVersions("runc", "ubuntu", "r2204")[0])
				ValidateSSHServiceEnabled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204FIPS(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 FIPS Gen1 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204FIPSContainerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{
					EnableFips1403Encryption: to.Ptr(true),
				}
				settings := vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.ProtectedSettings
				vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.Settings = settings
				vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.ProtectedSettings = nil
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", components.GetExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
				ValidateInstalledPackageVersion(ctx, s, "moby-runc", components.GetExpectedPackageVersions("runc", "ubuntu", "r2204")[0])
				ValidateSSHServiceEnabled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204Gen2FIPS(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 FIPS Gen2 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2FIPSContainerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{
					EnableFips1403Encryption: to.Ptr(true),
				}
				settings := vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.ProtectedSettings
				vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.Settings = settings
				vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.ProtectedSettings = nil
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", components.GetExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
				ValidateInstalledPackageVersion(ctx, s, "moby-runc", components.GetExpectedPackageVersions("runc", "ubuntu", "r2204")[0])
				ValidateSSHServiceEnabled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204Gen2FIPSTL(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 FIPS TrustedLaunch Gen2 VHD can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2FIPSTLContainerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
				vmss.Properties.AdditionalCapabilities = &armcompute.AdditionalCapabilities{
					EnableFips1403Encryption: to.Ptr(true),
				}
				settings := vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.ProtectedSettings
				vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.Settings = settings
				vmss.Properties.VirtualMachineProfile.ExtensionProfile.Extensions[0].Properties.ProtectedSettings = nil
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", components.GetExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
				ValidateInstalledPackageVersion(ctx, s, "moby-runc", components.GetExpectedPackageVersions("runc", "ubuntu", "r2204")[0])
				ValidateSSHServiceEnabled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_EntraIDSSH(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using Ubuntu 2204 VHD with Entra ID SSH can be properly bootstrapped and SSH private key authentication is disabled",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Enable Entra ID SSH authentication
				nbc.SSHStatus = datamodel.EntraIDSSH
			},
			SkipSSHConnectivityValidation: true, // Skip SSH connectivity validation since Entra ID SSH disables private key authentication
			SkipDefaultValidation:         true, // Skip default validation since it requires SSH connectivity
			Validator: func(ctx context.Context, s *Scenario) {
				// NOTE: Since Entra ID SSH disables pubkey authentication, we cannot use
				// the normal SSH-based validation functions that rely on private key authentication.
				// We can only validate that SSH private key authentication fails as expected.
				// The full E2E of Entra ID SSH scenario will be included in AKS RP's E2E test.

				// Validate Entra ID SSH configuration (tests that private key SSH fails)
				ValidatePubkeySSHDisabled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_EntraIDSSH_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using Ubuntu 2204 VHD with Entra ID SSH can be properly bootstrapped and SSH private key authentication is disabled",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.DisablePubkeyAuth = to.Ptr(true)
			},
			SkipSSHConnectivityValidation: true, // Skip SSH connectivity validation since Entra ID SSH disables private key authentication
			SkipDefaultValidation:         true, // Skip default validation since it requires SSH connectivity
			Validator: func(ctx context.Context, s *Scenario) {
				// NOTE: Since Entra ID SSH disables pubkey authentication, we cannot use
				// the normal SSH-based validation functions that rely on private key authentication.
				// We can only validate that SSH private key authentication fails as expected.
				// The full E2E of Entra ID SSH scenario will be included in AKS RP's E2E test.

				// Validate Entra ID SSH configuration (tests that private key SSH fails)
				ValidatePubkeySSHDisabled(ctx, s)
			},
		},
	})
}

func Test_AzureLinuxV3_DisableSSH(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using AzureLinuxV3 VHD with SSH disabled can be properly bootstrapped and SSH daemon is disabled",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.SSHStatus = datamodel.SSHOff
			},
			SkipSSHConnectivityValidation: true, // Skip SSH connectivity validation since SSH is down
			SkipDefaultValidation:         true, // Skip default validation since it requires SSH connectivity
			Validator: func(ctx context.Context, s *Scenario) {
				// Validate SSH daemon is disabled via RunCommand
				ValidateSSHServiceDisabled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_DisableSSH(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using Ubuntu 2204 VHD with SSH disabled can be properly bootstrapped and SSH daemon is disabled",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.SSHStatus = datamodel.SSHOff
			},
			SkipSSHConnectivityValidation: true, // Skip SSH connectivity validation since SSH is down
			SkipDefaultValidation:         true, // Skip default validation since it requires SSH connectivity
			Validator: func(ctx context.Context, s *Scenario) {
				// Validate SSH daemon is disabled via RunCommand
				ValidateSSHServiceDisabled(ctx, s)
			},
		},
	})
}

func Test_Flatcar_DisableSSH(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using Flatcar VHD with SSH disabled can be properly bootstrapped and SSH daemon is disabled",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDFlatcarGen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.SSHStatus = datamodel.SSHOff
			},
			SkipSSHConnectivityValidation: true, // Skip SSH connectivity validation since SSH is down
			SkipDefaultValidation:         true, // Skip default validation since it requires SSH connectivity
			Validator: func(ctx context.Context, s *Scenario) {
				// Validate SSH daemon is disabled via RunCommand
				ValidateSSHServiceDisabled(ctx, s)
			},
		},
	})
}

func Test_AzureLinuxV3_NetworkIsolatedCluster_NonAnonymousACR(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV3 (CgroupV2) VHD can be properly bootstrapped",
		Tags: Tags{
			NetworkIsolated: true,
			NonAnonymousACR: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetworkIsolated,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io/aks-managed-repository", config.PrivateACRNameNotAnon(config.Config.DefaultLocation)),
					},
				}
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.AgentPoolProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.K8sComponents.LinuxCredentialProviderURL = fmt.Sprintf(
					"https://packages.aks.azure.com/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz",
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion,
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/opt/azure", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_AzureLinuxV3_NetworkIsolated_Package_Install(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV3 VHD on ARM64 architecture can be properly bootstrapped",
		Tags: Tags{
			NetworkIsolated: true,
			NonAnonymousACR: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetworkIsolated,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeNone
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io/aks-managed-repository", config.PrivateACRNameNotAnon(config.Config.DefaultLocation)),
					},
				}
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.AgentPoolProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.K8sComponents.LinuxCredentialProviderURL = fmt.Sprintf(
					"https://packages.aks.azure.com/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz",
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion,
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["ShouldEnforceKubePMCInstall"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/run", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_Ubuntu2204_NetworkIsolatedCluster_NonAnonymousACR(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD and is network isolated can be properly bootstrapped",
		Tags: Tags{
			NetworkIsolated: true,
			NonAnonymousACR: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetworkIsolated,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io/aks-managed-repository", config.PrivateACRNameNotAnon(config.Config.DefaultLocation)),
					},
				}
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.AgentPoolProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.K8sComponents.LinuxCredentialProviderURL = fmt.Sprintf(
					"https://packages.aks.azure.com/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz",
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion,
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/opt/azure", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_Ubuntu2204Gen2_Containerd_NetworkIsolatedCluster_NoneCached(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD without k8s binary and is network isolated can be properly bootstrapped",
		Tags: Tags{
			NetworkIsolated: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetworkIsolated,
			VHD:     config.VHDUbuntu2204Gen2ContainerdNetworkIsolatedK8sNotCached,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io/aks-managed-repository", config.PrivateACRName(config.Config.DefaultLocation)),
					},
				}
				nbc.AgentPoolProfile.LocalDNSProfile = nil
				// intentionally using private acr url to get kube binaries
				nbc.AgentPoolProfile.KubernetesConfig.CustomKubeBinaryURL = fmt.Sprintf(
					"%s.azurecr.io/aks-managed-repository/oss/binaries/kubernetes/kubernetes-node:v%s-linux-amd64",
					config.PrivateACRName(config.Config.DefaultLocation),
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				nbc.EnableScriptlessCSECmd = false
			},
		},
	})
}

func Test_Ubuntu2204Gen2_Containerd_NetworkIsolatedCluster_NonAnonymousNoneCached(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD without k8s binary and is network isolated can be properly bootstrapped",
		Tags: Tags{
			NetworkIsolated: true,
			NonAnonymousACR: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetworkIsolated,
			VHD:     config.VHDUbuntu2204Gen2ContainerdNetworkIsolatedK8sNotCached,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io/aks-managed-repository", config.PrivateACRNameNotAnon(config.Config.DefaultLocation)),
					},
				}
				nbc.AgentPoolProfile.LocalDNSProfile = nil
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.AgentPoolProfile.KubernetesConfig.UseManagedIdentity = true
				// intentionally using private acr url to get kube binaries
				nbc.AgentPoolProfile.KubernetesConfig.CustomKubeBinaryURL = fmt.Sprintf(
					"%s.azurecr.io/aks-managed-repository/oss/binaries/kubernetes/kubernetes-node:v%s-linux-amd64",
					config.PrivateACRNameNotAnon(config.Config.DefaultLocation),
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				nbc.K8sComponents.LinuxCredentialProviderURL = fmt.Sprintf(
					"https://packages.aks.azure.com/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz",
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion,
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
				nbc.EnableScriptlessCSECmd = false
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/opt/azure", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_Ubuntu2204Gen2_Containerd_NetworkIsolatedCluster_NonAnonymousNoneCached_InstallPackage(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD without k8s binary and is network isolated can be properly bootstrapped",
		Tags: Tags{
			NetworkIsolated: true,
			NonAnonymousACR: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetworkIsolated,
			VHD:     config.VHDUbuntu2204Gen2ContainerdNetworkIsolatedK8sNotCached,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.OutboundType = datamodel.OutboundTypeBlock
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io/aks-managed-repository", config.PrivateACRNameNotAnon(config.Config.DefaultLocation)),
					},
				}
				nbc.AgentPoolProfile.LocalDNSProfile = nil
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.AgentPoolProfile.KubernetesConfig.UseManagedIdentity = true
				// intentionally using private acr url to get kube binaries
				nbc.AgentPoolProfile.KubernetesConfig.CustomKubeBinaryURL = fmt.Sprintf(
					"%s.azurecr.io/aks-managed-repository/oss/binaries/kubernetes/kubernetes-node:v%s-linux-amd64",
					config.PrivateACRNameNotAnon(config.Config.DefaultLocation),
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				nbc.K8sComponents.LinuxCredentialProviderURL = fmt.Sprintf(
					"https://packages.aks.azure.com/cloud-provider-azure/v%s/binaries/azure-acr-credential-provider-linux-amd64-v%s.tar.gz",
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion,
					nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion)
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
				nbc.EnableScriptlessCSECmd = false
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["ShouldEnforceKubePMCInstall"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/opt/azure", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_Ubuntu2204ARM64(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that an Ubuntu 2204 Node using ARM64 architecture can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Arm64Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"
				nbc.IsARM64 = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
		},
	})
}

func Test_Ubuntu2204_ArtifactStreaming(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using artifact streaming can be properly bootstrapepd",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.EnableArtifactStreaming = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/etc/overlaybd")
				ValidateSystemdUnitIsRunning(ctx, s, "overlaybd-snapshotter.service")
				ValidateSystemdUnitIsRunning(ctx, s, "overlaybd-tcmu.service")
				ValidateSystemdUnitIsRunning(ctx, s, "acr-mirror.service")
				ValidateSystemdUnitIsRunning(ctx, s, "containerd.service")
			},
		},
	})
}

func Test_Ubuntu2204_ArtifactStreaming_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a new ubuntu 2204 node using artifact streaming can be properly bootstrapepd",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.EnableArtifactStreaming = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/etc/overlaybd")
				ValidateSystemdUnitIsRunning(ctx, s, "overlaybd-snapshotter.service")
				ValidateSystemdUnitIsRunning(ctx, s, "overlaybd-tcmu.service")
				ValidateSystemdUnitIsRunning(ctx, s, "acr-mirror.service")
				ValidateSystemdUnitIsRunning(ctx, s, "containerd.service")
			},
		},
	})
}

func Test_Ubuntu2204_ChronyRestarts_Taints_And_Tolerations(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed. Also tests taints and tolerations",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.KubeletConfig["--register-with-taints"] = "testkey1=value1:NoSchedule,testkey2=value2:NoSchedule"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
				ValidateTaints(ctx, s, s.Runtime.NBC.KubeletConfig["--register-with-taints"])
			},
		},
	})
}

func Test_Ubuntu2204_ChronyRestarts_Taints_And_Tolerations_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that the chrony service restarts if it is killed. Also tests taints and tolerations",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.KubeletConfig.KubeletFlags["--register-with-taints"] = "testkey1=value1:NoSchedule,testkey2=value2:NoSchedule"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "Restart=always")
				ValidateFileHasContent(ctx, s, "/etc/systemd/system/chronyd.service.d/10-chrony-restarts.conf", "RestartSec=5")
				ServiceCanRestartValidator(ctx, s, "chronyd", 10)
				ValidateTaints(ctx, s, s.Runtime.AKSNodeConfig.KubeletConfig.KubeletFlags["--register-with-taints"])
			},
		},
	})
}

func Test_AzureLinuxV3_CustomCATrust(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Azure Linux V3 VHD can be properly bootstrapped and custom CA was correctly added",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
					CustomCATrustCerts: []string{
						encodedTestCert,
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/usr/share/pki/ca-trust-source/anchors")
			},
		},
	})
}

func Test_Ubuntu2204_CustomCATrust(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped and custom CA was correctly added",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.CustomCATrustConfig = &datamodel.CustomCATrustConfig{
					CustomCATrustCerts: []string{
						encodedTestCert,
					},
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/usr/local/share/ca-certificates/certs")
			},
		},
	})
}

func Test_Ubuntu2204_CustomCATrust_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD can be properly bootstrapped and custom CA was correctly added",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.CustomCaCerts = []string{encodedTestCert}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/usr/local/share/ca-certificates/certs")
			},
		},
	})
}

func Test_Ubuntu2204_CustomSysctls(t *testing.T) {
	customSysctls := map[string]string{
		"net.ipv4.ip_local_port_range":       "32768 65535",
		"net.netfilter.nf_conntrack_max":     "2097152",
		"net.netfilter.nf_conntrack_buckets": "524288",
		"net.ipv4.tcp_keepalive_intvl":       "90",
		"net.ipv4.ip_local_reserved_ports":   "65330",
	}
	customContainerdUlimits := map[string]string{
		"LimitMEMLOCK": "75000",
		"LimitNOFILE":  "1048",
	}
	RunScenario(t, &Scenario{
		Description: "tests that an ubuntu 2204 VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				customLinuxConfig := &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_max"])),
						NetNetfilterNfConntrackBuckets: to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_buckets"])),
						NetIpv4IpLocalPortRange:        customSysctls["net.ipv4.ip_local_port_range"],
						NetIpv4TcpkeepaliveIntvl:       to.Ptr(toolkit.StrToInt32(customSysctls["net.ipv4.tcp_keepalive_intvl"])),
					},
					UlimitConfig: &datamodel.UlimitConfig{
						MaxLockedMemory: "75000",
						NoFile:          "1048",
					},
				}
				nbc.AgentPoolProfile.CustomLinuxOSConfig = customLinuxConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateUlimitSettings(ctx, s, customContainerdUlimits)
				ValidateSysctlConfig(ctx, s, customSysctls)
			},
		},
	})
}

func Test_Ubuntu2204_CustomSysctls_Scriptless(t *testing.T) {
	customSysctls := map[string]string{
		"net.ipv4.ip_local_port_range":       "32768 65535",
		"net.netfilter.nf_conntrack_max":     "2097152",
		"net.netfilter.nf_conntrack_buckets": "524288",
		"net.ipv4.tcp_keepalive_intvl":       "90",
		"net.ipv4.ip_local_reserved_ports":   "65330",
	}
	customContainerdUlimits := map[string]string{
		"LimitMEMLOCK": "75000",
		"LimitNOFILE":  "1048",
	}
	RunScenario(t, &Scenario{
		Description: "tests that an ubuntu 2204 VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				customLinuxOsConfig := &aksnodeconfigv1.CustomLinuxOsConfig{
					SysctlConfig: &aksnodeconfigv1.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_max"])),
						NetNetfilterNfConntrackBuckets: to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_buckets"])),
						NetIpv4IpLocalPortRange:        to.Ptr(customSysctls["net.ipv4.ip_local_port_range"]),
						NetIpv4TcpkeepaliveIntvl:       to.Ptr(toolkit.StrToInt32(customSysctls["net.ipv4.tcp_keepalive_intvl"])),
					},
					UlimitConfig: &aksnodeconfigv1.UlimitConfig{
						MaxLockedMemory: to.Ptr(customContainerdUlimits["LimitMEMLOCK"]),
						NoFile:          to.Ptr(customContainerdUlimits["LimitNOFILE"]),
					},
				}
				config.CustomLinuxOsConfig = customLinuxOsConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateUlimitSettings(ctx, s, customContainerdUlimits)
				ValidateSysctlConfig(ctx, s, customSysctls)
			},
		},
	})
}

func Test_Ubuntu2204_GPUNC(t *testing.T) {
	runScenarioUbuntu2204GPU(t, "Standard_NC6s_v3")
}

func Test_Ubuntu2204_GPUA100(t *testing.T) {
	runScenarioUbuntu2204GPU(t, "Standard_NC24ads_A100_v4")
}

func Test_Ubuntu2204_GPUA10(t *testing.T) {
	runScenarioUbuntuGRID(t, "Standard_NV6ads_A10_v5")
}

// Returns config for the 'gpu' E2E scenario
func runScenarioUbuntu2204GPU(t *testing.T, vmSize string) {
	RunScenario(t, &Scenario{
		Description: fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2204 VHD can be properly bootstrapped", vmSize),
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateServicesDoNotRestartKubelet(ctx, s)
			},
		},
	})
}

func runScenarioUbuntuGRID(t *testing.T, vmSize string) {
	RunScenario(t, &Scenario{
		Description: fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2204 VHD can be properly bootstrapped, and that the GRID license is valid", vmSize),
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateNvidiaGRIDLicenseValid(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateServicesDoNotRestartKubelet(ctx, s)
				ValidateNvidiaPersistencedRunning(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_GPUA10_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests scriptless installer that a GPU-enabled node using the Ubuntu 2204 VHD with grid driver can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateServicesDoNotRestartKubelet(ctx, s)
			},
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.VmSize = "Standard_NV6ads_A10_v5"
				config.GpuConfig.ConfigGpuDriver = true
				config.GpuConfig.GpuDevicePlugin = false
				config.GpuConfig.EnableNvidia = to.Ptr(true)
			},
		},
	})
}

func Test_Ubuntu2204_GPUGridDriver(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2204 VHD with grid driver can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NV6ads_A10_v5"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NV6ads_A10_v5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateNvidiaSMIInstalled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_GPUNoDriver(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2204 VHD opting for skipping gpu driver installation can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Tags = map[string]*string{
					// deliberately case mismatched to agentbaker logic to check case insensitivity
					"SkipGPUDriverInstall": to.Ptr("true"),
				}
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNvidiaSMINotInstalled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_GPUNoDriver_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2204 VHD opting for skipping gpu driver installation can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.VmSize = "Standard_NC6s_v3"
				config.GpuConfig.ConfigGpuDriver = true
				config.GpuConfig.GpuDevicePlugin = false
				config.GpuConfig.EnableNvidia = to.Ptr(true)
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				// this vmss tag is needed since there is a logic in cse_main.sh otherwise the test will fail
				vmss.Tags = map[string]*string{
					// deliberately case mismatched to agentbaker logic to check case insensitivity
					"SkipGPUDriverInstall": to.Ptr("true"),
				}
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNvidiaSMINotInstalled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_PrivateKubePkg(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD that was built with private kube packages can be properly bootstrapped with the specified kube version",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.25.6"
				nbc.K8sComponents.LinuxPrivatePackageURL = "https://privatekube.blob.core.windows.net/kubernetes/v1.25.6-hotfix.20230612/binaries/v1.25.6-hotfix.20230612.tar.gz"
				nbc.AgentPoolProfile.LocalDNSProfile = nil
				nbc.EnableScriptlessCSECmd = false
			},
		},
	})
}

// These tests were created to verify that the apt-get call in downloadContainerdFromVersion is not executed.
// The code path is not hit in either of these tests. In the future, testing with some kind of firewall to ensure no egress
// calls are made would be beneficial for airgap testing.

// Combine old e2e tests for scenario Ubuntu2204_ContainerdURL and Ubuntu2204_IMDSRestrictionFilterTable
func Test_Ubuntu2204_ContainerdURL_IMDSRestrictionFilterTable(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: `tests that a node using the Ubuntu 2204 VHD with the ContainerdPackageURL override bootstraps with the provided URL and not the components.json containerd version,
		              tests that the imds restriction filter table is properly set`,
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerdPackageURL = "https://packages.microsoft.com/ubuntu/22.04/prod/pool/main/m/moby-containerd/moby-containerd_1.6.9+azure-ubuntu22.04u1_amd64.deb"
				nbc.EnableIMDSRestriction = true
				nbc.InsertIMDSRestrictionRuleToMangleTable = false
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "containerd", "1.6.9")
			},
		},
	})
}

// Combine e2e scriptless tests for scenario Ubuntu2204_ContainerdURL and Ubuntu2204_IMDSRestrictionFilterTable
func Test_Ubuntu2204_ContainerdURL_IMDSRestrictionFilterTable_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: `tests that a node using the Ubuntu 2204 VHD with the ContainerdPackageURL override the provided URL and not the components.json containerd version,
		              tests that the imds restriction filter table is properly set`,
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.ContainerdConfig.ContainerdPackageUrl = "https://packages.microsoft.com/ubuntu/22.04/prod/pool/main/m/moby-containerd/moby-containerd_1.6.9+azure-ubuntu22.04u1_amd64.deb"
				config.ImdsRestrictionConfig = &aksnodeconfigv1.ImdsRestrictionConfig{
					EnableImdsRestriction:                  true,
					InsertImdsRestrictionRuleToMangleTable: false,
				}
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "containerd", "1.6.9")
			},
		},
	})
}

func Test_Ubuntu2204_ContainerdHasCurrentVersion(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node using an Ubuntu2204 VHD and the ContainerdVersion override bootstraps with the correct components.json containerd version and ignores the override",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", components.GetExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
			},
		},
	})
}

func Test_AzureLinux_Skip_Binary_Cleanup(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that an AzureLinux node will skip binary cleanup and can be properly bootstrapped",
		Config: Config{
			Cluster:                ClusterKubenet,
			VHD:                    config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["SkipBinaryCleanup"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateMultipleKubeProxyVersionsExist(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node on ubuntu 2204 bootstrapped with kubelet serving certificate rotation enabled will disable certificate rotation due to nodepool tags",
		Config: Config{
			Cluster:                ClusterKubenet,
			VHD:                    config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
		},
	})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags_CustomKubeletConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node on ubuntu 2204 bootstrapped with custom kubelet config and kubelet serving certificate rotation enabled will disable certificate rotation due to nodepool tags",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// to force kubelet config file
				customKubeletConfig := &datamodel.CustomKubeletConfig{
					FailSwapOn:           to.Ptr(true),
					AllowedUnsafeSysctls: &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
		},
	})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags_CustomKubeletConfig_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node on ubuntu 2204 bootstrapped with custom kubelet config and kubelet serving certificate rotation enabled will disable certificate rotation due to nodepool tags",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.KubeletConfig.EnableKubeletConfigFile = true
				config.KubeletConfig.KubeletConfigFileConfig.FailSwapOn = to.Ptr(true)
				config.KubeletConfig.KubeletConfigFileConfig.AllowedUnsafeSysctls = []string{"kernel.msg*", "net.ipv4.route.min_pmtu"}
				config.KubeletConfig.KubeletConfigFileConfig.ServerTlsBootstrap = true
				config.KubeletConfig.KubeletConfigFileConfig.FeatureGates = map[string]bool{"RotateKubeletServerCertificate": true}
				config.EnableUnattendedUpgrade = false
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
		},
	})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags_AlreadyDisabled(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node on ubuntu 2204 bootstrapped with kubelet serving certificate rotation disabled will disable certificate rotation regardless of nodepool tags",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
		},
	})
}

func Test_Ubuntu2204_DisableKubeletServingCertificateRotationWithTags_AlreadyDisabled_CustomKubeletConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node on ubuntu 2204 bootstrapped with kubelet serving certificate rotation disabled and custom kubelet config will disable certificate rotation regardless of nodepool tags",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// to force kubelet config file
				customKubeletConfig := &datamodel.CustomKubeletConfig{
					FailSwapOn:           to.Ptr(true),
					AllowedUnsafeSysctls: &[]string{"kernel.msg*", "net.ipv4.route.min_pmtu"},
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["aks-disable-kubelet-serving-certificate-rotation"] = to.Ptr("true")
			},
		},
	})
}

func Test_Ubuntu2204_MessageOfTheDay(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that a node on ubuntu 2204 bootstrapped and message of the day is properly added to the node",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.MessageOfTheDay = "Zm9vYmFyDQo=" // base64 for foobar
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/motd", "foobar")
				ValidateFileHasContent(ctx, s, "/etc/update-motd.d/99-aks-custom-motd", "cat /etc/motd")
			},
		},
	})
}

func Test_AzureLinuxV3_MessageOfTheDay(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV3 can be bootstrapped and message of the day is added to the node",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.MessageOfTheDay = "Zm9vYmFyDQo=" // base64 for foobar
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/motd", "foobar")
				ValidateFileHasContent(ctx, s, "/etc/dnf/automatic.conf", "emit_via = stdio")
			},
		},
	})
}

func Test_AzureLinuxV3_MessageOfTheDay_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV3 can be bootstrapped and message of the day is added to the node",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.MessageOfTheDay = "Zm9vYmFyDQo=" // base64 for foobar
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateFileHasContent(ctx, s, "/etc/motd", "foobar")
				ValidateFileHasContent(ctx, s, "/etc/dnf/automatic.conf", "emit_via = stdio")
			},
		},
	})
}

func Test_AzureLinuxV3_MA35D(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using AzureLinuxV3 can support MA35D SKU",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_NM16ads_MA35D"
				nbc.AgentPoolProfile.VMSize = "Standard_NM16ads_MA35D"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NM16ads_MA35D")
				vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings.Placement = to.Ptr(armcompute.DiffDiskPlacementCacheDisk)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/sys/devices/virtual/misc/ama_transcoder0")
				ValidateNonEmptyDirectory(ctx, s, "/opt/amd/ama/ma35/")
				ValidateSystemdUnitIsRunning(ctx, s, "amdama-device-plugin.service")
				ValidateNodeAdvertisesGPUResources(ctx, s, 1, "squat.ai/amdama")
			},
		},
		// No MA35D GPU capacity in West US, so using East US
		Location:         "eastus",
		K8sSystemPoolSKU: "Standard_D2s_v3",
	})
}

func Test_AzureLinuxV3_MA35D_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using AzureLinuxV3 can support MA35D SKU",
		Tags: Tags{
			Scriptless: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.VmSize = "Standard_NM16ads_MA35D"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NM16ads_MA35D")
				vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.DiffDiskSettings.Placement = to.Ptr(armcompute.DiffDiskPlacementCacheDisk)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNonEmptyDirectory(ctx, s, "/sys/devices/virtual/misc/ama_transcoder0")
				ValidateNonEmptyDirectory(ctx, s, "/opt/amd/ama/ma35/")
				ValidateSystemdUnitIsRunning(ctx, s, "amdama-device-plugin.service")
				ValidateNodeAdvertisesGPUResources(ctx, s, 1, "squat.ai/amdama")
			},
		},
		// No MA35D GPU capacity in West US, so using East US
		Location:         "eastus",
		K8sSystemPoolSKU: "Standard_D2s_v3",
	})
}

func Test_AzureLinuxV3LocalDns_Disabled_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a AzureLinuxV3 can be bootstrapped with localdns disabled",
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDAzureLinuxV3Gen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.LocalDnsProfile = &aksnodeconfigv1.LocalDnsProfile{
					EnableLocalDns: false,
				}
			},
			SkipDefaultValidation: true,
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateLocalDNSService(ctx, s, "disabled")
				ValidateLocalDNSResolution(ctx, s, "168.63.129.16")
			},
		},
	})
}

func Test_AzureLinuxV3_CustomSysctls(t *testing.T) {
	customSysctls := map[string]string{
		"net.ipv4.ip_local_port_range":       "32768 62535",
		"net.netfilter.nf_conntrack_max":     "2097152",
		"net.netfilter.nf_conntrack_buckets": "524288",
		"net.ipv4.tcp_keepalive_intvl":       "90",
		"net.ipv4.ip_local_reserved_ports":   "",
	}
	customContainerdUlimits := map[string]string{
		"LimitMEMLOCK": "75000",
		"LimitNOFILE":  "1048",
	}
	RunScenario(t, &Scenario{
		Description: "tests that a AzureLinuxV3 (CgroupV2) VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				customLinuxConfig := &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_max"])),
						NetNetfilterNfConntrackBuckets: to.Ptr(toolkit.StrToInt32(customSysctls["net.netfilter.nf_conntrack_buckets"])),
						NetIpv4IpLocalPortRange:        customSysctls["net.ipv4.ip_local_port_range"],
						NetIpv4TcpkeepaliveIntvl:       to.Ptr(toolkit.StrToInt32(customSysctls["net.ipv4.tcp_keepalive_intvl"])),
					},
					UlimitConfig: &datamodel.UlimitConfig{
						MaxLockedMemory: customContainerdUlimits["LimitMEMLOCK"],
						NoFile:          customContainerdUlimits["LimitNOFILE"],
					},
				}
				nbc.AgentPoolProfile.CustomLinuxOSConfig = customLinuxConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateUlimitSettings(ctx, s, customContainerdUlimits)
				ValidateSysctlConfig(ctx, s, customSysctls)
			},
		},
	})
}

func Test_Ubuntu2204_KubeletCustomConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			KubeletCustomConfig: true,
		},
		Description: "tests that a node on ubuntu 2204 bootstrapped with kubelet custom config for seccomp set to non default values",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-ubuntu-containerd-22.04-gen2"
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-containerd-22.04-gen2"
				customKubeletConfig := &datamodel.CustomKubeletConfig{
					SeccompDefault: to.Ptr(true),
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = customKubeletConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				kubeletConfigFilePath := "/etc/default/kubeletconfig.json"
				ValidateFileHasContent(ctx, s, kubeletConfigFilePath, `"seccompDefault": true`)
				ValidateKubeletHasFlags(ctx, s, kubeletConfigFilePath)
			},
		},
	})
}

func Test_AzureLinuxV3_KubeletCustomConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			KubeletCustomConfig: true,
		},
		Description: "tests that a node on azure linux v3 bootstrapped with kubelet custom config for seccomp set to non default values",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-azurelinux-v3-gen2"
				nbc.AgentPoolProfile.Distro = "aks-azurelinux-v3-gen2"
				customKubeletConfig := &datamodel.CustomKubeletConfig{
					SeccompDefault: to.Ptr(true),
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = customKubeletConfig
			},
			Validator: func(ctx context.Context, s *Scenario) {
				kubeletConfigFilePath := "/etc/default/kubeletconfig.json"
				ValidateFileHasContent(ctx, s, kubeletConfigFilePath, `"seccompDefault": true`)
				ValidateKubeletHasFlags(ctx, s, kubeletConfigFilePath)
				ValidateInstalledPackageVersion(ctx, s, "containerd2", components.GetExpectedPackageVersions("containerd", "azurelinux", "v3.0")[0])
			},
		},
	})
}

func Test_AzureLinuxV3_KubeletCustomConfig_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			KubeletCustomConfig: true,
		},
		Description: "tests that a node on azure linux v3 bootstrapped with kubelet custom config for seccomp set to non default values",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.KubeletConfig.KubeletConfigFileConfig.SeccompDefault = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				kubeletConfigFilePath := "/etc/default/kubeletconfig.json"
				ValidateFileHasContent(ctx, s, kubeletConfigFilePath, `"seccompDefault": true`)
				ValidateKubeletHasFlags(ctx, s, kubeletConfigFilePath)
				ValidateInstalledPackageVersion(ctx, s, "containerd2", components.GetExpectedPackageVersions("containerd", "azurelinux", "v3.0")[0])
			},
		},
	})
}

func Test_AzureLinuxV3_GPU(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using a AzureLinuxV3 (CgroupV2) VHD can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
		},
	})
}

func Test_AzureLinuxV3_GPUAzureCNI(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "AzureLinux V3 (CgroupV2) gpu scenario on cluster configured with Azure CNI",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.KubernetesConfig.NetworkPlugin = string(armcontainerservice.NetworkPluginAzure)
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
		},
	})
}

func Test_AzureLinuxV3_GPUAzureCNI_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "AzureLinux V3 (CgroupV2) gpu scenario on cluster configured with Azure CNI",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterAzureNetwork,
			VHD:     config.VHDAzureLinuxV3Gen2,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
				config.NetworkConfig.NetworkPlugin = aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE
				config.VmSize = "Standard_NC6s_v3"
				config.GpuConfig.ConfigGpuDriver = true
				config.GpuConfig.GpuDevicePlugin = false
				config.GpuConfig.EnableNvidia = to.Ptr(true)
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
		},
	})
}

func Test_Ubuntu2204ARM64_KubeletCustomConfig(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			KubeletCustomConfig: true,
		},
		Description: "tests that a node on ubuntu 2204 ARM64 bootstrapped with kubelet custom config",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Arm64Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.IsARM64 = true
				nbc.AgentPoolProfile.Distro = "aks-ubuntu-arm64-containerd-22.04-gen2"
				nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_V5"
				nbc.AgentPoolProfile.VMSize = "Standard_D2pds_V5"

				customKubeletConfig := &datamodel.CustomKubeletConfig{
					SeccompDefault: to.Ptr(true),
				}
				nbc.AgentPoolProfile.CustomKubeletConfig = customKubeletConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomKubeletConfig = customKubeletConfig
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},

			Validator: func(ctx context.Context, s *Scenario) {
				kubeletConfigFilePath := "/etc/default/kubeletconfig.json"
				ValidateFileHasContent(ctx, s, kubeletConfigFilePath, `"seccompDefault": true`)
				ValidateKubeletHasFlags(ctx, s, kubeletConfigFilePath)
			},
		},
	})
}

func Test_Ubuntu2404Gen2(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2404 VHD can be properly bootstrapped with containerd v2",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				containerdVersions := components.GetExpectedPackageVersions("containerd", "ubuntu", "r2404")
				runcVersions := components.GetExpectedPackageVersions("runc", "ubuntu", "r2404")
				ValidateContainerd2Properties(ctx, s, containerdVersions)
				ValidateRuncVersion(ctx, s, runcVersions)
				ValidateContainerRuntimePlugins(ctx, s)
				ValidateSSHServiceEnabled(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2404Gen2_McrChinaCloud_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			MockAzureChinaCloud: true,
			Scriptless:          true,
		},
		Description: "Tests that a node using the Ubuntu 2404 VHD can be properly bootstrapped with containerd v2",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			AKSNodeConfigMutator: func(config *aksnodeconfigv1.Configuration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["E2EMockAzureChinaCloud"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateDirectoryContent(ctx, s, "/etc/containerd/certs.d/mcr.azk8s.cn", []string{"hosts.toml"})
			},
		},
	})
}

func Test_Ubuntu2404Gen2_McrChinaCloud(t *testing.T) {
	RunScenario(t, &Scenario{
		Tags: Tags{
			MockAzureChinaCloud: true,
		},
		Description: "Tests that a node using the Ubuntu 2404 VHD can be properly bootstrapped with containerd v2",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["E2EMockAzureChinaCloud"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				containerdVersions := components.GetExpectedPackageVersions("containerd", "ubuntu", "r2404")
				runcVersions := components.GetExpectedPackageVersions("runc", "ubuntu", "r2404")
				ValidateContainerd2Properties(ctx, s, containerdVersions)
				ValidateRuncVersion(ctx, s, runcVersions)
				ValidateContainerRuntimePlugins(ctx, s)
				ValidateSSHServiceEnabled(ctx, s)
				ValidateDirectoryContent(ctx, s, "/etc/containerd/certs.d/mcr.azk8s.cn", []string{"hosts.toml"})
			},
		},
	})
}

func Test_Ubuntu2204_SecureTLSBootstrapping_BootstrapToken_Fallback(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using an Ubuntu 2204 Gen2 VHD can be properly bootstrapped even if secure TLS bootstrapping fails",
		Tags: Tags{
			BootstrapTokenFallback: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{
					Enabled:                true,
					Deadline:               (10 * time.Second).String(),
					UserAssignedIdentityID: "invalid", // use an unexpected user-assigned identity ID to force a secure TLS bootstrapping failure
				}
			},
		},
	})
}

func Test_Ubuntu2404_SecureTLSBootstrapping_BootstrapToken_Fallback(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using an Ubuntu 2404 Gen2 VHD can be properly bootstrapped even if secure TLS bootstrapping fails",
		Tags: Tags{
			BootstrapTokenFallback: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{
					Enabled:                true,
					Deadline:               (10 * time.Second).String(),
					UserAssignedIdentityID: "invalid", // use an unexpected user-assigned identity ID to force a secure TLS bootstrapping failure
				}
			},
		},
	})
}

func Test_Ubuntu2404Gen2_GPUNoDriver(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a GPU-enabled node using the Ubuntu 2404 VHD opting for skipping gpu driver installation can be properly bootstrapped",
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = "Standard_NC6s_v3"
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Tags = map[string]*string{
					// deliberately case mismatched to agentbaker logic to check case insensitivity
					"SkipGPUDriverInstall": to.Ptr("true"),
				}
				vmss.SKU.Name = to.Ptr("Standard_NC6s_v3")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				containerdVersions := components.GetExpectedPackageVersions("containerd", "ubuntu", "r2404")
				runcVersions := components.GetExpectedPackageVersions("runc", "ubuntu", "r2404")

				ValidateNvidiaSMINotInstalled(ctx, s)
				ValidateContainerd2Properties(ctx, s, containerdVersions)
				ValidateRuncVersion(ctx, s, runcVersions)
			},
		},
	})
}

func Test_Ubuntu2404Gen1(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2404 VHD can be properly bootstrapped with containerd v2",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen1Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			Validator: func(ctx context.Context, s *Scenario) {
				containerdVersions := components.GetExpectedPackageVersions("containerd", "ubuntu", "r2404")
				runcVersions := components.GetExpectedPackageVersions("runc", "ubuntu", "r2404")
				ValidateContainerd2Properties(ctx, s, containerdVersions)
				ValidateRuncVersion(ctx, s, runcVersions)
			},
		},
	})
}

func Test_Ubuntu2404ARM(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2404 VHD can be properly bootstrapped with containerd v2",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404ArmContainerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr("Standard_D2pds_V5")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				containerdVersions := components.GetExpectedPackageVersions("containerd", "ubuntu", "r2404")
				runcVersions := components.GetExpectedPackageVersions("runc", "ubuntu", "r2404")
				ValidateContainerd2Properties(ctx, s, containerdVersions)
				ValidateRuncVersion(ctx, s, runcVersions)
			},
		},
	})
}

func Test_Random_VHD_With_Latest_Kubernetes_Version(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using a Random VHD can be properly bootstrapped with the latest kubernetes version",
		Config: Config{
			Cluster: ClusterLatestKubernetesVersion,
			VHD:     config.GetRandomLinuxAMD64VHD(),
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
		},
	})
}

func runScenarioUbuntu2404GRID(t *testing.T, vmSize string) {
	RunScenario(t, &Scenario{
		Description: fmt.Sprintf("Tests that a GPU-enabled node with VM size %s using an Ubuntu 2404 VHD can be properly bootstrapped, and that the GRID license is valid", vmSize),
		Tags: Tags{
			GPU: true,
		},
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.VMSize = vmSize
				nbc.ConfigGPUDriverIfNeeded = true
				nbc.EnableGPUDevicePluginIfNeeded = false
				nbc.EnableNvidia = true
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.SKU.Name = to.Ptr(vmSize)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Ensure nvidia-modprobe install does not restart kubelet and temporarily cause node to be unschedulable
				ValidateNvidiaModProbeInstalled(ctx, s)
				ValidateNvidiaGRIDLicenseValid(ctx, s)
				ValidateKubeletHasNotStopped(ctx, s)
				ValidateServicesDoNotRestartKubelet(ctx, s)
				ValidateNvidiaPersistencedRunning(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2404_GPUA10(t *testing.T) {
	runScenarioUbuntu2404GRID(t, "Standard_NV6ads_A10_v5")
}

func Test_Ubuntu2404_NPD_Basic(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Test that a node with AKS VM Extension enabled can report simulated node problem detector events",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				extension, err := createVMExtensionLinuxAKSNode(vmss.Location)
				require.NoError(t, err, "creating AKS VM extension")
				vmss.Properties = addVMExtensionToVMSS(vmss.Properties, extension)
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateNodeProblemDetector(ctx, s)
				ValidateNPDFilesystemCorruption(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2404_GPU_H100(t *testing.T) {
	RunScenario(t, runScenarioGPUNPD(t, "Standard_ND96isr_H100_v5", "uaenorth", ""))
}

func Test_Ubuntu2404_GPU_A100(t *testing.T) {
	RunScenario(t, runScenarioGPUNPD(t, "Standard_ND96asr_v4", "southcentralus", "Standard_D2s_v3"))
}

func Test_AzureLinux3_PMC_Install(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "tests that an AzureLinux node will install kube pkgs from PMC and can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["ShouldEnforceKubePMCInstall"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
			},
		},
	})
}

func Test_Ubuntu2204_PMC_Install(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using the Ubuntu 2204 VHD and install kube pkgs from PMC can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Check that we don't leak these secrets if they're
				// set (which they mostly aren't in these scenarios).
				nbc.ContainerService.Properties.CertificateProfile.ClientPrivateKey = "client cert private key"
				nbc.ContainerService.Properties.ServicePrincipalProfile.Secret = "SP secret"
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["ShouldEnforceKubePMCInstall"] = to.Ptr("true")
			},
			Validator: func(ctx context.Context, s *Scenario) {
				ValidateInstalledPackageVersion(ctx, s, "moby-containerd", components.GetExpectedPackageVersions("containerd", "ubuntu", "r2204")[0])
				ValidateInstalledPackageVersion(ctx, s, "moby-runc", components.GetExpectedPackageVersions("runc", "ubuntu", "r2204")[0])
				ValidateSSHServiceEnabled(ctx, s)
			},
		},
	})
}

func Test_AzureLinux3OSGuard_PMC_Install(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that a node using an Azure Linux V3 OS Guard VHD and install kube pkgs from PMC can be properly bootstrapped",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDAzureLinux3OSGuard,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.LocalDNSProfile = nil
			},
			Validator: func(ctx context.Context, s *Scenario) {},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
				if vmss.Tags == nil {
					vmss.Tags = map[string]*string{}
				}
				vmss.Tags["ShouldEnforceKubePMCInstall"] = to.Ptr("true")
			},
		},
	})
}

func Test_Ubuntu2404_VHDCaching(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "T",
		Config: Config{
			Cluster:                ClusterKubenet,
			VHD:                    config.VHDUbuntu2204Gen2Containerd,
			VHDCaching:             true,
			BootstrapConfigMutator: EmptyBootstrapConfigMutator,
			Validator: func(ctx context.Context, s *Scenario) {
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				// If the VHD has incorrect settings (like network misconfiguration)
				// deploying more than one VM may expose the issue.
				// This check is not always reliable, since only one VM is created per test run in the current framework.
				// Therefore, tests may incorrectly pass more often than they fail in these cases.
				vmss.SKU.Capacity = to.Ptr[int64](2)
			},
		},
	})
}

func Test_AzureLinuxV3_AppArmor(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that AppArmor is properly enabled and configured on Azure Linux V3 nodes",
		Config: Config{
			Cluster:                ClusterKubenet,
			VHD:                    config.VHDAzureLinuxV3Gen2,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {},
			Validator: func(ctx context.Context, s *Scenario) {
				// Validate that AppArmor kernel module is loaded and service is active
				ValidateAppArmorBasic(ctx, s)
			},
		},
	})
}

func Test_Ubuntu2204Gen2_ImagePullIdentityBinding_Enabled(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that credential provider config includes identity binding when ServiceAccountImagePullProfile is enabled",
		Config: Config{
			Cluster: ClusterLatestKubernetesVersion,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Enforce Kubernetes 1.34.0 for ServiceAccountImagePullProfile testing
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.34.0"
				// Enable ServiceAccountImagePullProfile with test values
				nbc.ContainerService.Properties.ServiceAccountImagePullProfile = &datamodel.ServiceAccountImagePullProfile{
					Enabled:           true,
					DefaultClientID:   "test-client-id-12345",
					DefaultTenantID:   "test-tenant-id-67890",
					LocalAuthoritySNI: "test.sni.local",
				}
				// Set kubelet flags to enable credential provider config generation
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Verify credential provider config file exists
				ValidateFileExists(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml")

				// Verify the config contains identity binding arguments
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-client-id=test-client-id-12345")
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-tenant-id=test-tenant-id-67890")
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-sni-name=test.sni.local")

				// Verify the config contains the identity binding token attributes section
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "serviceAccountTokenAudience: api://AKSIdentityBinding")
			},
		},
	})
}

func Test_Ubuntu2204Gen2_ImagePullIdentityBinding_Disabled(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that credential provider config excludes identity binding when ServiceAccountImagePullProfile is disabled",
		Config: Config{
			Cluster: ClusterLatestKubernetesVersion,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Enforce Kubernetes 1.34.0 for ServiceAccountImagePullProfile testing
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.34.0"
				// Explicitly disable ServiceAccountImagePullProfile
				nbc.ContainerService.Properties.ServiceAccountImagePullProfile = &datamodel.ServiceAccountImagePullProfile{
					Enabled: false,
				}
				// Set kubelet flags to enable credential provider config generation
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Verify credential provider config file exists
				ValidateFileExists(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml")

				// Verify the config does NOT contain identity binding arguments
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-client-id")
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-tenant-id")
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-sni-name")
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "serviceAccountTokenAudience: api://AKSIdentityBinding")
			},
		},
	})
}

func Test_Ubuntu2204Gen2_ImagePullIdentityBinding_EnabledWithoutDefaultIDs(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that credential provider config includes identity binding without default client/tenant IDs when not specified",
		Config: Config{
			Cluster: ClusterLatestKubernetesVersion,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Enforce Kubernetes 1.34.0 for ServiceAccountImagePullProfile testing
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.34.0"
				// Enable ServiceAccountImagePullProfile without default client/tenant IDs
				nbc.ContainerService.Properties.ServiceAccountImagePullProfile = &datamodel.ServiceAccountImagePullProfile{
					Enabled:           true,
					DefaultClientID:   "", // Empty - should not generate --ib-default-client-id flag
					DefaultTenantID:   "", // Empty - should not generate --ib-default-tenant-id flag
					LocalAuthoritySNI: "test.sni.local",
				}
				// Set kubelet flags to enable credential provider config generation
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Verify credential provider config file exists
				ValidateFileExists(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml")

				// Verify the config contains identity binding token attributes
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "serviceAccountTokenAudience: api://AKSIdentityBinding")
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-sni-name=test.sni.local")

				// Verify the config does NOT contain default client/tenant ID flags
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-client-id")
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-tenant-id")
			},
		},
	})
}

func Test_Ubuntu2204Gen2_ImagePullIdentityBinding_NetworkIsolated(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that credential provider config includes identity binding in network isolated (NI) clusters",
		Tags: Tags{
			NetworkIsolated: true,
			NonAnonymousACR: true,
		},
		Config: Config{
			Cluster: ClusterAzureBootstrapProfileCache,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				// Enforce Kubernetes 1.34.0 for ServiceAccountImagePullProfile testing
				nbc.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = "1.34.0"
				// Enable ServiceAccountImagePullProfile with test values
				nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
					PrivateEgress: &datamodel.PrivateEgress{
						Enabled:                 true,
						ContainerRegistryServer: fmt.Sprintf("%s.azurecr.io/aks-managed-repository", config.PrivateACRNameNotAnon(config.Config.DefaultLocation)),
					},
				}
				nbc.ContainerService.Properties.ServiceAccountImagePullProfile = &datamodel.ServiceAccountImagePullProfile{
					Enabled:           true,
					DefaultClientID:   "ni-test-client-id",
					DefaultTenantID:   "ni-test-tenant-id",
					LocalAuthoritySNI: "ni.test.sni.local",
				}
				// Set kubelet flags to enable credential provider config generation
				nbc.KubeletConfig["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				nbc.KubeletConfig["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
				nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.UseManagedIdentity = true
				nbc.AgentPoolProfile.KubernetesConfig.UseManagedIdentity = true
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Verify credential provider config file exists
				ValidateFileExists(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml")

				// Verify the config contains identity binding arguments for NI cluster
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-client-id=ni-test-client-id")
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-tenant-id=ni-test-tenant-id")
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-sni-name=ni.test.sni.local")
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "serviceAccountTokenAudience: api://AKSIdentityBinding")

				// Verify outbound check was skipped (network isolated)
				ValidateDirectoryContent(ctx, s, "/opt/azure", []string{"outbound-check-skipped"})
			},
		},
	})
}

func Test_Ubuntu2204Gen2_ImagePullIdentityBinding_Enabled_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that credential provider config includes identity binding when ServiceAccountImagePullProfile is enabled in scriptless mode",
		Config: Config{
			Cluster: ClusterLatestKubernetesVersion,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(aksConfig *aksnodeconfigv1.Configuration) {
				// Enforce Kubernetes 1.34.0 for ServiceAccountImagePullProfile testing
				aksConfig.KubernetesVersion = "1.34.0"
				// Enable ServiceAccountImagePullProfile with test values
				aksConfig.ServiceAccountImagePullProfile = &aksnodeconfigv1.ServiceAccountImagePullProfile{
					Enabled:           true,
					DefaultClientId:   "test-client-id-12345",
					DefaultTenantId:   "test-tenant-id-67890",
					LocalAuthoritySni: "test.sni.local",
				}
				// Set kubelet flags to enable credential provider
				if aksConfig.KubeletConfig == nil {
					aksConfig.KubeletConfig = &aksnodeconfigv1.KubeletConfig{}
				}
				if aksConfig.KubeletConfig.KubeletFlags == nil {
					aksConfig.KubeletConfig.KubeletFlags = make(map[string]string)
				}
				aksConfig.KubeletConfig.KubeletFlags["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				aksConfig.KubeletConfig.KubeletFlags["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Verify aks-node-controller completed successfully
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")

				// Verify credential provider config file exists
				ValidateFileExists(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml")

				// Verify the config contains identity binding arguments
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-client-id=test-client-id-12345")
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-tenant-id=test-tenant-id-67890")
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-sni-name=test.sni.local")

				// Verify the config contains the identity binding token attributes section
				ValidateFileHasContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "serviceAccountTokenAudience: api://AKSIdentityBinding")
			},
		},
	})
}

func Test_Ubuntu2204Gen2_ImagePullIdentityBinding_Disabled_Scriptless(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests that credential provider config excludes identity binding when ServiceAccountImagePullProfile is disabled in scriptless mode",
		Config: Config{
			Cluster: ClusterLatestKubernetesVersion,
			VHD:     config.VHDUbuntu2204Gen2Containerd,
			AKSNodeConfigMutator: func(aksConfig *aksnodeconfigv1.Configuration) {
				// Enforce Kubernetes 1.34.0 for ServiceAccountImagePullProfile testing
				aksConfig.KubernetesVersion = "1.34.0"
				// Disable ServiceAccountImagePullProfile
				aksConfig.ServiceAccountImagePullProfile = &aksnodeconfigv1.ServiceAccountImagePullProfile{
					Enabled:           false,
					DefaultClientId:   "should-not-appear-client-id",
					DefaultTenantId:   "should-not-appear-tenant-id",
					LocalAuthoritySni: "should.not.appear.sni",
				}
				// Set kubelet config to enable credential provider
				if aksConfig.KubeletConfig == nil {
					aksConfig.KubeletConfig = &aksnodeconfigv1.KubeletConfig{}
				}
				if aksConfig.KubeletConfig.KubeletFlags == nil {
					aksConfig.KubeletConfig.KubeletFlags = make(map[string]string)
				}
				aksConfig.KubeletConfig.KubeletFlags["--image-credential-provider-config"] = "/var/lib/kubelet/credential-provider-config.yaml"
				aksConfig.KubeletConfig.KubeletFlags["--image-credential-provider-bin-dir"] = "/var/lib/kubelet/credential-provider"
			},
			Validator: func(ctx context.Context, s *Scenario) {
				// Verify aks-node-controller completed successfully
				ValidateFileHasContent(ctx, s, "/var/log/azure/aks-node-controller.log", "aks-node-controller finished successfully")

				// Verify credential provider config file exists
				ValidateFileExists(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml")

				// Verify the config does NOT contain identity binding arguments
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-client-id")
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-default-tenant-id")
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "--ib-sni-name")
				ValidateFileExcludesContent(ctx, s, "/var/lib/kubelet/credential-provider-config.yaml", "serviceAccountTokenAudience: api://AKSIdentityBinding")
			},
		},
	})
}

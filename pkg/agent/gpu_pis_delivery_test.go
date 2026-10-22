package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPISGPUBakedHelperDelivery(t *testing.T) {
	for _, distro := range []datamodel.Distro{datamodel.AKSUbuntuContainerd2204Gen2, datamodel.AKSUbuntuContainerd2404Gen2} {
		for _, mode := range []struct {
			preProvisionOnly bool
			phase1           bool
		}{{true, true}, {true, false}, {false, false}} {
			t.Run(fmt.Sprintf("%s/PreProvisionOnly=%t/Phase1=%t", distro, mode.preProvisionOnly, mode.phase1), func(t *testing.T) {
				pool := &datamodel.AgentPoolProfile{
					Name: "agent2", VMSize: "Standard_NC4as_T4_v3", StorageProfile: "ManagedDisks",
					OSType: datamodel.Linux, AvailabilityProfile: datamodel.VirtualMachineScaleSets,
					VnetSubnetID: "/subscriptions/test/resourceGroups/test/providers/Microsoft.Network/virtualNetworks/test/subnets/default",
					Distro:       distro,
				}
				properties := &datamodel.Properties{
					OrchestratorProfile:     &datamodel.OrchestratorProfile{OrchestratorType: datamodel.Kubernetes, OrchestratorVersion: "1.32.1", KubernetesConfig: &datamodel.KubernetesConfig{}},
					HostedMasterProfile:     &datamodel.HostedMasterProfile{DNSPrefix: "fixture"},
					LinuxProfile:            &datamodel.LinuxProfile{AdminUsername: "azureuser"},
					ServicePrincipalProfile: &datamodel.ServicePrincipalProfile{ClientID: "fixture", Secret: "fixture"},
					AgentPoolProfiles:       []*datamodel.AgentPoolProfile{pool},
				}
				properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{KeyData: "fixture-key"}}
				config := &datamodel.NodeBootstrappingConfiguration{
					ContainerService: &datamodel.ContainerService{Location: "southcentralus", Type: "Microsoft.ContainerService/ManagedClusters", Properties: properties},
					CloudSpecConfig:  datamodel.AzurePublicCloudSpecForTest, K8sComponents: &datamodel.K8sComponents{},
					AgentPoolProfile: pool, TenantID: "tenantID", SubscriptionID: "subID",
					ResourceGroupName: "resourceGroupName", UserAssignedIdentityClientID: "userAssignedID",
					KubeletConfig:       map[string]string{"--cluster-domain": "cluster.local", "--cluster-dns": "10.0.0.10", "--cloud-provider": "azure", "--max-pods": "110"},
					PrimaryScaleSetName: "aks-agent2-36873793-vmss", PreProvisionOnly: mode.preProvisionOnly,
					EnableNvidia: false, ConfigGPUDriverIfNeeded: false,
					EnableGPUDevicePluginIfNeeded: false, EnableScriptlessCSECmd: mode.phase1,
				}

				require.False(t, supportsScriptlessProvisioning(config))
				generator := InitializeTemplateGenerator()
				command := generator.getNodeBootstrappingCmd(config)
				require.Contains(t, command, fmt.Sprintf(`PRE_PROVISION_ONLY="%t"`, mode.preProvisionOnly))
				require.Contains(t, command, "GPU_NODE=false")
				require.Contains(t, command, `/bin/bash /opt/azure/containers/provision_start.sh`)
				require.NotContains(t, command, "aks-node-controller provision-wait")

				payload := generator.getLinuxNodeBootstrappingPayload(config)
				compressed, err := base64.StdEncoding.DecodeString(payload)
				require.NoError(t, err)
				decoded, err := getGzipDecodedValue(compressed)
				require.NoError(t, err)
				t.Logf("PIS T4 payload: yaml=%d gzip=%d base64=%d gzip-margin=%d base64-margin=%d", len(decoded), len(compressed), len(payload), 65535-len(compressed), 87380-len(payload))
				require.LessOrEqual(t, len(payload), 87380)
				require.LessOrEqual(t, len(compressed), 65535)
				var document cloudInit
				require.NoError(t, yaml.Unmarshal(decoded, &document))
				installerFound := false
				for _, file := range document.WriteFiles {
					require.NotEqual(t, "/opt/azure/containers/provision_installs_ubuntu_gpu.sh", file.Path)
					if file.Path == "/opt/azure/containers/provision_installs_distro.sh" {
						installerFound = true
						script, err := getGzipDecodedValue([]byte(file.Content))
						require.NoError(t, err)
						require.NotContains(t, string(script), "graceBlackwellConfigIsUnmodified()")
						require.NotContains(t, string(script), "cleanUpGPUDriversForBasePrep()")
						require.Contains(t, string(script), "cleanUpGPUDrivers()")
					}
				}
				if mode.phase1 {
					require.Len(t, document.WriteFiles, 1)
					require.Equal(t, "/opt/azure/containers/scriptless-cse-overrides.txt", document.WriteFiles[0].Path)
				} else {
					require.Greater(t, len(document.WriteFiles), 1)
					require.True(t, installerFound)
				}
			})
		}
	}
}

func TestPISGPUBakedHelperProvenance(t *testing.T) {
	const helper = "parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu_gpu.sh"
	root := filepath.Join("..", "..")
	for _, variant := range []string{"base", "cvm", "arm64-gen2", "arm64-gb"} {
		data, err := os.ReadFile(filepath.Join(root, "vhdbuilder", "packer", "vhd-image-builder-"+variant+".json"))
		require.NoError(t, err)
		var image struct {
			Provisioners []struct{ Type, Source, Destination string }
		}
		require.NoError(t, json.Unmarshal(data, &image))
		matches := 0
		for _, provisioner := range image.Provisioners {
			if provisioner.Type == "file" && provisioner.Source == helper &&
				provisioner.Destination == "/home/packer/provision_installs_ubuntu_gpu.sh" {
				matches++
			}
		}
		require.Equal(t, 1, matches, variant)
	}
	// Execute the raw Packer source, not the comment-stripped CustomData copy.
	config := filepath.Join(t.TempDir(), "nvidia.conf")
	require.NoError(t, os.WriteFile(config, []byte("fixture"), 0600))
	command := exec.Command("bash", "-c", `
source "$1"
graceBlackwellInstalledDriverPackages() { :; }
graceBlackwellPackageManagedDriverRemains() { return 1; }
graceBlackwellConfigIsUnmodified() {
    printf '%d\n' "$#"
    printf '%s\n' "$@"
    return 1
}
GB_NVIDIA_MODPROBE_CONFIG_FILE="$2"
cleanUpGraceBlackwellGPUDriver unused unused unused
`, "gpu-helper-test", filepath.Join(root, helper), config)
	// A present config reaches the comparison without executing teardown.
	output, err := command.CombinedOutput()
	require.Error(t, err)
	require.True(t, strings.HasPrefix(string(output), "8\n"), string(output))
	require.Contains(t, string(output), "# Explicitly load nvidia_peermem at boot for GPUDirect RDMA.")
}

func TestGPUAMADeliveryIsDistroScoped(t *testing.T) {
	distros := append([]datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSUbuntuContainerd2404Gen2,
		datamodel.AKSACLGen2TL,
		datamodel.AKSFlatcarGen2,
		datamodel.CustomizedImage,
		datamodel.CustomizedImageKata,
		datamodel.CustomizedImageLinuxGuard,
		datamodel.CustomizedImageTrustedLaunch,
	}, datamodel.AvailableAzureLinuxDistros...)
	for _, distro := range distros {
		t.Run(string(distro), func(t *testing.T) {
			config := newNodeCustomDataRenderConfig(distro)
			encoded := getBase64EncodedGzippedCustomScript(kubernetesCSEConfigGPU, config)
			compressed, err := base64.StdEncoding.DecodeString(encoded)
			require.NoError(t, err)
			script, err := getGzipDecodedValue(compressed)
			require.NoError(t, err)
			custom := distro == datamodel.CustomizedImage || distro == datamodel.CustomizedImageKata ||
				distro == datamodel.CustomizedImageLinuxGuard || distro == datamodel.CustomizedImageTrustedLaunch
			keepImplementation := distro.IsAzureLinuxDistro() || custom
			require.Equal(t, keepImplementation, strings.Contains(string(script), "dnf_install_amd_ama_core_package()"))
			require.Equal(t, keepImplementation, strings.Contains(string(script), `dnf repoquery -y --available "amd-ama-driver-*"`))
			require.Contains(t, string(script), "setupAmdAma()")
			syntax := exec.Command("bash", "-n")
			syntax.Stdin = strings.NewReader(string(script))
			output, err := syntax.CombinedOutput()
			require.NoError(t, err, string(output))
			if keepImplementation {
				raw := exec.Command("bash", "-c", `source "$1"; declare -f setupAmdAma dnf_install_amd_ama_core_package`,
					"gpu-raw-test", filepath.Join("..", "..", "parts", kubernetesCSEConfigGPU))
				rawFunctions, err := raw.CombinedOutput()
				require.NoError(t, err, string(rawFunctions))
				rendered := exec.Command("bash", "-c", string(script)+"\ndeclare -f setupAmdAma dnf_install_amd_ama_core_package")
				renderedFunctions, err := rendered.CombinedOutput()
				require.NoError(t, err, string(renderedFunctions))
				require.Equal(t, string(rawFunctions), string(renderedFunctions))
			} else {
				command := exec.Command("bash", "-c", string(script)+`
isARM64() { echo 0; }
isAzureLinux() { return 1; }
setupAmdAma
`)
				output, err := command.CombinedOutput()
				require.NoError(t, err, string(output))
				require.Empty(t, output)
			}
		})
	}
}

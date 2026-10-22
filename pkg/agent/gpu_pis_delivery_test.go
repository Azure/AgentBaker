package agent

import (
	"encoding/base64"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPISGPUCustomDataFitsAndUsesBakedHelper(t *testing.T) {
	for _, mode := range []struct {
		preProvisionOnly bool
		phase1           bool
	}{
		{preProvisionOnly: true, phase1: true},
		{preProvisionOnly: true},
		{},
	} {
		config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2404Gen2)
		pool := config.AgentPoolProfile
		pool.Name, pool.VMSize = "agent2", "Standard_NC4as_T4_v3"
		pool.StorageProfile = "ManagedDisks"
		pool.AvailabilityProfile = datamodel.VirtualMachineScaleSets
		pool.VnetSubnetID = "/subscriptions/test/resourceGroups/test/providers/Microsoft.Network/virtualNetworks/test/subnets/default"
		properties := config.ContainerService.Properties
		properties.LinuxProfile = &datamodel.LinuxProfile{AdminUsername: "azureuser"}
		properties.LinuxProfile.SSH.PublicKeys = []datamodel.PublicKey{{KeyData: "fixture-key"}}
		properties.ServicePrincipalProfile = &datamodel.ServicePrincipalProfile{ClientID: "fixture", Secret: "fixture"}
		config.TenantID, config.SubscriptionID = "tenantID", "subID"
		config.ResourceGroupName, config.UserAssignedIdentityClientID = "resourceGroupName", "userAssignedID"
		config.PrimaryScaleSetName = "aks-agent2-36873793-vmss"
		config.KubeletConfig = map[string]string{"--cluster-domain": "cluster.local", "--cluster-dns": "10.0.0.10", "--cloud-provider": "azure", "--max-pods": "110"}
		config.PreProvisionOnly = mode.preProvisionOnly
		config.EnableScriptlessCSECmd = mode.phase1

		payload := InitializeTemplateGenerator().getLinuxNodeBootstrappingPayload(config)
		compressed, err := base64.StdEncoding.DecodeString(payload)
		require.NoError(t, err)
		decoded, err := getGzipDecodedValue(compressed)
		require.NoError(t, err)
		require.LessOrEqual(t, len(compressed), 65535)
		require.LessOrEqual(t, len(payload), 87380)

		var document cloudInit
		require.NoError(t, yaml.Unmarshal(decoded, &document))
		var installer bool
		for _, file := range document.WriteFiles {
			require.NotEqual(t, "/opt/azure/containers/provision_installs_ubuntu_gpu.sh", file.Path)
			if file.Path == "/opt/azure/containers/provision_installs_distro.sh" {
				installer = true
				script, err := getGzipDecodedValue([]byte(file.Content))
				require.NoError(t, err)
				require.Contains(t, string(script), "cleanUpGPUDrivers()")
			}
		}
		if mode.phase1 {
			require.Equal(t, "/opt/azure/containers/scriptless-cse-overrides.txt", document.WriteFiles[0].Path)
		} else {
			require.True(t, installer)
		}
	}
}

func TestGPUAMARenderingKeepsAzureLinuxImplementation(t *testing.T) {
	config := newNodeCustomDataRenderConfig(datamodel.AKSAzureLinuxV3Gen2)
	payload := getBase64EncodedGzippedCustomScript(kubernetesCSEConfigGPU, config)
	compressed, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(t, err)
	rendered, err := getGzipDecodedValue(compressed)
	require.NoError(t, err)

	raw := exec.Command("bash", "-c", `source "$1"; declare -f setupAmdAma dnf_install_amd_ama_core_package`,
		"gpu-raw-test", filepath.Join("..", "..", "parts", kubernetesCSEConfigGPU))
	rawFunctions, err := raw.CombinedOutput()
	require.NoError(t, err, string(rawFunctions))
	renderedCmd := exec.Command("bash", "-c", string(rendered)+"\ndeclare -f setupAmdAma dnf_install_amd_ama_core_package")
	renderedFunctions, err := renderedCmd.CombinedOutput()
	require.NoError(t, err, string(renderedFunctions))
	require.Equal(t, string(rawFunctions), string(renderedFunctions))
}

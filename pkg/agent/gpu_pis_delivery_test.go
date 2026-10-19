package agent

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPISGPUPhase1UsesBakedScripts(t *testing.T) {
	for _, preProvisionOnly := range []bool{true, false} {
		t.Run(fmt.Sprintf("PreProvisionOnly=%t", preProvisionOnly), func(t *testing.T) {
			pool := &datamodel.AgentPoolProfile{
				Name: "agent2", VMSize: "Standard_NC4as_T4_v3", StorageProfile: "ManagedDisks",
				OSType: datamodel.Linux, AvailabilityProfile: datamodel.VirtualMachineScaleSets,
				VnetSubnetID: "/subscriptions/test/resourceGroups/test/providers/Microsoft.Network/virtualNetworks/test/subnets/default",
				Distro:       datamodel.AKSUbuntuContainerd2404Gen2,
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
				PrimaryScaleSetName: "aks-agent2-36873793-vmss", PreProvisionOnly: preProvisionOnly,
				EnableNvidia: false, ConfigGPUDriverIfNeeded: false,
				EnableGPUDevicePluginIfNeeded: false, EnableScriptlessCSECmd: true,
			}

			// Phase 1 uses baked CSE scripts, independently of ANC's later phases.
			require.False(t, supportsScriptlessProvisioning(config))
			generator := InitializeTemplateGenerator()
			command := generator.getNodeBootstrappingCmd(config)
			require.Contains(t, command, fmt.Sprintf(`PRE_PROVISION_ONLY="%t"`, preProvisionOnly))
			require.Contains(t, command, "GPU_NODE=false")
			require.Contains(t, command, `/bin/bash /opt/azure/containers/provision_start.sh`)
			require.NotContains(t, command, "aks-node-controller provision-wait")

			payload := generator.getLinuxNodeBootstrappingPayload(config)
			compressed, err := base64.StdEncoding.DecodeString(payload)
			require.NoError(t, err)
			decoded, err := getGzipDecodedValue(compressed)
			require.NoError(t, err)
			t.Logf("phase-1 PIS T4 payload: yaml=%d gzip=%d base64=%d", len(decoded), len(compressed), len(payload))
			require.LessOrEqual(t, len(payload), 87380)
			require.LessOrEqual(t, len(compressed), 65535)
			var document cloudInit
			require.NoError(t, yaml.Unmarshal(decoded, &document))
			require.Len(t, document.WriteFiles, 1)
			require.Equal(t, "/opt/azure/containers/scriptless-cse-overrides.txt", document.WriteFiles[0].Path)
		})
	}
}

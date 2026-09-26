package scenario

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestConfigureOSSKarpenterClusterModel(t *testing.T) {
	model := getBaseClusterModel("karpenter", "westus3", "Standard_D2ds_v5")

	configureOSSKarpenterClusterModel(model)

	require.NotNil(t, model.Properties.NetworkProfile)
	assert.Equal(t, armcontainerservice.NetworkPluginAzure, *model.Properties.NetworkProfile.NetworkPlugin)
	assert.Equal(t, armcontainerservice.NetworkPluginModeOverlay, *model.Properties.NetworkProfile.NetworkPluginMode)
	assert.Equal(t, armcontainerservice.NetworkDataplaneCilium, *model.Properties.NetworkProfile.NetworkDataplane)
	assert.Equal(t, armcontainerservice.NetworkPolicyCilium, *model.Properties.NetworkProfile.NetworkPolicy)
}

func TestRegisteredOSSKarpenterScenarioUsesCustomDataOverride(t *testing.T) {
	var found *Scenario
	for _, candidate := range List() {
		if candidate.Name == "Ubuntu2204_OSS_Karpenter_CSE_Compatibility" {
			found = candidate
			break
		}
	}
	require.NotNil(t, found)
	assert.Same(t, config.VHDUbuntu2204Gen2Containerd, found.VHD)
	assert.NotNil(t, found.CustomDataOverride)
	assert.Nil(t, found.ClusterTest)
}

func newOSSKarpenterTestScenario() *Scenario {
	networkPlugin := armcontainerservice.NetworkPluginAzure
	networkPolicy := armcontainerservice.NetworkPolicyCilium
	clusterName := "cluster"
	location := "westus3"
	nodeRG := "MC_rg_cluster_westus3"
	orchestratorVersion := "1.30.0"

	s := &Scenario{
		Config: Config{VHD: config.VHDUbuntu2204Gen2Containerd},
		Runtime: &ScenarioRuntime{
			Kube: &Kubeclient{RESTConfig: &rest.Config{Host: "https://cluster.example"}},
			Cluster: &Cluster{
				TenantID: "tenant",
				SubnetID: "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet",
				Model: &armcontainerservice.ManagedCluster{
					Name:     &clusterName,
					Location: &location,
					Properties: &armcontainerservice.ManagedClusterProperties{
						NodeResourceGroup: &nodeRG,
						NetworkProfile: &armcontainerservice.NetworkProfile{
							NetworkPlugin: &networkPlugin,
							NetworkPolicy: &networkPolicy,
						},
					},
				},
				ClusterParams: &ClusterParams{
					CACert:         []byte("test-ca-cert"),
					BootstrapToken: "token",
					FQDN:           "cluster.example",
				},
			},
			NBC: &datamodel.NodeBootstrappingConfiguration{
				UserAssignedIdentityClientID: "client-id",
				ContainerService: &datamodel.ContainerService{
					Properties: &datamodel.Properties{
						ClusterID: "12345678",
						OrchestratorProfile: &datamodel.OrchestratorProfile{
							OrchestratorType:    datamodel.Kubernetes,
							OrchestratorVersion: orchestratorVersion,
						},
					},
				},
			},
		},
	}
	return s
}

func TestRenderOSSKarpenterCustomDataProducesScript(t *testing.T) {
	s := newOSSKarpenterTestScenario()

	customData, cseCmd, err := renderOSSKarpenterCustomData(context.Background(), s)

	require.NoError(t, err)
	assert.Empty(t, cseCmd, "OSS Karpenter's scriptless mode uses no separate CSE VM extension")
	assert.NotEmpty(t, customData)
}

func TestRenderOSSKarpenterCustomDataRequiresCompleteRuntime(t *testing.T) {
	s := &Scenario{Runtime: &ScenarioRuntime{}}

	_, _, err := renderOSSKarpenterCustomData(context.Background(), s)

	assert.Error(t, err)
}

func TestRenderOSSKarpenterCustomDataEncodesCACertBase64(t *testing.T) {
	s := newOSSKarpenterTestScenario()

	customData, _, err := renderOSSKarpenterCustomData(context.Background(), s)
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(customData)
	require.NoError(t, err, "rendered CustomData is expected to be base64-encoded")

	expected := base64.StdEncoding.EncodeToString(s.Runtime.Cluster.ClusterParams.CACert)
	assert.Contains(t, string(decoded), expected)
}

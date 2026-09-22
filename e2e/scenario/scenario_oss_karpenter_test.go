package scenario

import (
	"context"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestConfigureOSSKarpenterClusterModel(t *testing.T) {
	model := getBaseClusterModel("karpenter", "westus3", "Standard_D2ds_v5")

	configureOSSKarpenterClusterModel(model)

	require.NotNil(t, model.Properties.NetworkProfile)
	assert.Equal(t, armcontainerservice.NetworkPluginAzure, *model.Properties.NetworkProfile.NetworkPlugin)
	assert.Equal(t, armcontainerservice.NetworkPluginModeOverlay, *model.Properties.NetworkProfile.NetworkPluginMode)
	assert.Equal(t, armcontainerservice.NetworkDataplaneCilium, *model.Properties.NetworkProfile.NetworkDataplane)
	assert.Equal(t, armcontainerservice.NetworkPolicyCilium, *model.Properties.NetworkProfile.NetworkPolicy)
	require.NotNil(t, model.Properties.OidcIssuerProfile)
	assert.True(t, *model.Properties.OidcIssuerProfile.Enabled)
	require.NotNil(t, model.Properties.SecurityProfile)
	require.NotNil(t, model.Properties.SecurityProfile.WorkloadIdentity)
	assert.True(t, *model.Properties.SecurityProfile.WorkloadIdentity.Enabled)
}

func TestNewOSSKarpenterNodeClassSelectsExactVHDWithoutSpecImageID(t *testing.T) {
	const imageID = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/galleries/gallery/images/2204gen2containerd/versions/1.2.3"

	nodeClass := newOSSKarpenterNodeClass("test", imageID)

	assert.Equal(t, imageID, nodeClass.GetAnnotations()[ossKarpenterImageIDAnnotation])
	imageFamily, found, err := unstructuredString(nodeClass.Object, "spec", "imageFamily")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Ubuntu2204", imageFamily)
	_, found, err = unstructuredString(nodeClass.Object, "spec", "imageID")
	require.NoError(t, err)
	assert.False(t, found, "upstream v1.14.2 does not expose spec.imageID")
}

func TestNewOSSKarpenterNodePoolTargetsRunAndVMSize(t *testing.T) {
	run := ossKarpenterRun{
		NodeClassName: "class",
		NodePoolName:  "pool",
		RunLabelValue: "run",
	}

	nodePool := newOSSKarpenterNodePool(run, "Standard_D2ds_v5")

	assert.Equal(t, "pool", nodePool.GetName())
	className, found, err := unstructuredString(nodePool.Object, "spec", "template", "spec", "nodeClassRef", "name")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "class", className)
	runLabel, found, err := unstructuredString(nodePool.Object, "spec", "template", "metadata", "labels", ossKarpenterRunLabel)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "run", runLabel)
	consolidateAfter, found, err := unstructuredString(nodePool.Object, "spec", "disruption", "consolidateAfter")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "Never", consolidateAfter)

	requirements, found, err := unstructuredSlice(nodePool.Object, "spec", "template", "spec", "requirements")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, requirementHasValue(requirements, "node.kubernetes.io/instance-type", "Standard_D2ds_v5"))
	assert.True(t, requirementHasValue(requirements, "karpenter.sh/capacity-type", "on-demand"))
}

func TestDeleteOSSKarpenterNodeClaimsTargetsOnlyNodePool(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "karpenter.sh", Version: "v1", Kind: "NodeClaim"}
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind("NodeClaimList"), &unstructured.UnstructuredList{})
	claim := func(name, nodePool string) *unstructured.Unstructured {
		object := &unstructured.Unstructured{}
		object.SetGroupVersionKind(gvk)
		object.SetName(name)
		object.SetLabels(map[string]string{"karpenter.sh/nodepool": nodePool})
		return object
	}
	dynamic := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		claim("target", "target-pool"),
		claim("other", "other-pool"),
	).Build()

	require.NoError(t, deleteOSSKarpenterNodeClaims(context.Background(), &Kubeclient{Dynamic: dynamic}, "target-pool"))

	claims := &unstructured.UnstructuredList{}
	claims.SetGroupVersionKind(gvk.GroupVersion().WithKind("NodeClaimList"))
	require.NoError(t, dynamic.List(context.Background(), claims))
	require.Len(t, claims.Items, 1)
	assert.Equal(t, "other", claims.Items[0].GetName())
}

func TestNewOSSKarpenterWorkloadCannotUseSystemPool(t *testing.T) {
	run := ossKarpenterRun{
		Namespace:     "workload",
		PodName:       "inflate",
		RunLabelValue: "run",
	}

	pod := newOSSKarpenterWorkload(run)

	assert.Equal(t, "run", pod.Spec.NodeSelector[ossKarpenterRunLabel])
	require.Len(t, pod.Spec.Containers, 1)
	assert.Equal(t, "mcr.microsoft.com/oss/kubernetes/pause:3.6", pod.Spec.Containers[0].Image)
	assert.Equal(t, "1", pod.Spec.Containers[0].Resources.Requests.Cpu().String())
	assert.Equal(t, "256Mi", pod.Spec.Containers[0].Resources.Requests.Memory().String())
}

func TestSameAzureResourceIDIsCaseAndTrailingSlashInsensitive(t *testing.T) {
	assert.True(t, sameAzureResourceID(
		"/subscriptions/SUB/resourceGroups/RG/providers/Microsoft.Compute/galleries/G/images/I/versions/1.2.3/",
		"/subscriptions/sub/resourceGroups/rg/providers/microsoft.compute/galleries/g/images/i/versions/1.2.3",
	))
	assert.False(t, sameAzureResourceID("/subscriptions/a", "/subscriptions/b"))
}

func TestOSSKarpenterPatchDoesNotVendorCSE(t *testing.T) {
	patch := strings.ToLower(string(ossKarpenterSourcePatch))
	assert.NotContains(t, patch, "cse_cmd.sh.gtpl")
	assert.NotContains(t, patch, "pkg/providers/imagefamily/bootstrap/")
	assert.Contains(t, patch, ossKarpenterImageIDAnnotation)
	assert.Contains(t, patch, "operator.getconfig()", "out-of-cluster controller must use its kubeconfig")
}

func TestRegisteredOSSKarpenterScenarioUsesClusterLifecycle(t *testing.T) {
	var found *Scenario
	for _, candidate := range List() {
		if candidate.Name == "Ubuntu2204_OSS_Karpenter_CSE_Compatibility" {
			found = candidate
			break
		}
	}
	require.NotNil(t, found)
	assert.Same(t, config.VHDUbuntu2204Gen2Containerd, found.VHD)
	assert.NotNil(t, found.ClusterTest)
}

func unstructuredString(object map[string]any, fields ...string) (string, bool, error) {
	current := any(object)
	for _, field := range fields {
		mapping, ok := current.(map[string]any)
		if !ok {
			return "", false, nil
		}
		next, ok := mapping[field]
		if !ok {
			return "", false, nil
		}
		current = next
	}
	value, ok := current.(string)
	return value, ok, nil
}

func unstructuredSlice(object map[string]any, fields ...string) ([]any, bool, error) {
	current := any(object)
	for _, field := range fields {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		next, ok := mapping[field]
		if !ok {
			return nil, false, nil
		}
		current = next
	}
	value, ok := current.([]any)
	return value, ok, nil
}

func requirementHasValue(requirements []any, key, value string) bool {
	for _, raw := range requirements {
		requirement, ok := raw.(map[string]any)
		if !ok || requirement["key"] != key {
			continue
		}
		values, ok := requirement["values"].([]any)
		if !ok {
			return false
		}
		for _, candidate := range values {
			if candidate == value {
				return true
			}
		}
	}
	return false
}

func TestOSSKarpenterControllerEnvironment(t *testing.T) {
	networkPlugin := armcontainerservice.NetworkPluginAzure
	networkPluginMode := armcontainerservice.NetworkPluginModeOverlay
	networkPolicy := armcontainerservice.NetworkPolicyCilium
	networkDataplane := armcontainerservice.NetworkDataplaneCilium
	clusterName := "cluster"
	location := "westus3"
	nodeRG := "MC_rg_cluster_westus3"
	dnsIP := "172.16.0.10"
	resourceID := "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/kubelet"
	clientID := "client"
	originalSubscription := config.Config.SubscriptionID
	originalPublicKey := config.SysSSHPublicKey
	t.Cleanup(func() {
		config.Config.SubscriptionID = originalSubscription
		config.SysSSHPublicKey = originalPublicKey
	})
	config.Config.SubscriptionID = "sub"
	config.SysSSHPublicKey = []byte("ssh-rsa test")

	s := &Scenario{
		Runtime: &ScenarioRuntime{
			Kube: &Kubeclient{RESTConfig: &rest.Config{Host: "https://cluster.example"}},
			Cluster: &Cluster{
				Model: &armcontainerservice.ManagedCluster{
					Name:     &clusterName,
					Location: &location,
					Properties: &armcontainerservice.ManagedClusterProperties{
						NodeResourceGroup: &nodeRG,
						NetworkProfile: &armcontainerservice.NetworkProfile{
							NetworkPlugin:     &networkPlugin,
							NetworkPluginMode: &networkPluginMode,
							NetworkPolicy:     &networkPolicy,
							NetworkDataplane:  &networkDataplane,
							DNSServiceIP:      &dnsIP,
						},
					},
				},
				KubeletIdentity: &armcontainerservice.UserAssignedIdentity{
					ClientID:   &clientID,
					ResourceID: &resourceID,
				},
				SubnetID:         "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/subnet",
				VNetResourceGUID: "vnet-guid",
				ClusterParams:    &ClusterParams{BootstrapToken: "token"},
			},
		},
	}

	environment, err := ossKarpenterControllerEnvironment(s)
	require.NoError(t, err)
	env := map[string]string{}
	for _, assignment := range environment {
		key, value, ok := strings.Cut(assignment, "=")
		require.True(t, ok)
		env[key] = value
	}
	assert.Equal(t, "true", env["AGENTBAKER_E2E_USE_AZURE_CLI"])
	assert.Equal(t, "aksscriptless", env["PROVISION_MODE"])
	assert.Equal(t, "false", env["USE_SIG"])
	assert.Equal(t, "false", env["DISABLE_LEADER_ELECTION"])
	assert.Equal(t, "kube-system", env["LEADER_ELECTION_NAMESPACE"])
	assert.Equal(t, "https://cluster.example", env["CLUSTER_ENDPOINT"])
	assert.Equal(t, resourceID, env["NODE_IDENTITIES"])
	assert.Equal(t, "ssh-rsa test azureuser", env["SSH_PUBLIC_KEY"])
}

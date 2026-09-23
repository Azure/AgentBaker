package scenario

import (
	"context"
	"fmt"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
)

// ossKarpenterAzureVersion/ossKarpenterAzureCommit record which pinned
// karpenter-provider-azure release this scenario targets. e2e/go.mod's
// dependency on github.com/Azure/karpenter-provider-azure (used by
// renderOSSKarpenterCustomData in oss_karpenter_render.go) must be pinned to
// the same commit. Bump both together:
//
//	cd e2e && go get github.com/Azure/karpenter-provider-azure@<new commit> && go mod tidy
const (
	ossKarpenterAzureVersion = "v1.14.2"
	ossKarpenterAzureCommit  = "d1552b7e96e3d3bf44acc55be5786b25cdbfaaf4"
)

var ClusterOSSKarpenter = cachedFunc(clusterOSSKarpenter)

func clusterOSSKarpenter(ctx context.Context, request ClusterRequest) (*Cluster, error) {
	model, err := getLatestKubernetesVersionClusterModel(ctx, "abe2e-oss-karpenter-v1", request.Location, request.K8sSystemPoolSKU)
	if err != nil {
		return nil, fmt.Errorf("getting OSS Karpenter cluster model: %w", err)
	}
	configureOSSKarpenterClusterModel(model)
	return prepareCluster(ctx, model, false, false)
}

// configureOSSKarpenterClusterModel matches the network configuration real OSS
// Karpenter deployments commonly use (Azure CNI Overlay + Cilium), so the
// values fed into the rendered CSE (NETWORK_PLUGIN, NETWORK_POLICY, ...) are
// realistic rather than AgentBaker's own scenario defaults.
func configureOSSKarpenterClusterModel(model *armcontainerservice.ManagedCluster) {
	azureOverlayNetworkClusterModelMutator(model)
	model.Properties.NetworkProfile.NetworkDataplane = to.Ptr(armcontainerservice.NetworkDataplaneCilium)
	model.Properties.NetworkProfile.NetworkPolicy = to.Ptr(armcontainerservice.NetworkPolicyCilium)
}

var _ = Register(&Scenario{
	Name: "Ubuntu2204_OSS_Karpenter_CSE_Compatibility",
	Description: "Provisions a node through AgentBaker's normal E2E VMSS lifecycle (VMSS creation, wait for " +
		"Ready, default pod-scheduling validation), but replaces AgentBaker's own rendered CSE with the exact " +
		"cse_cmd.sh the pinned upstream karpenter-provider-azure controller would generate for the same " +
		"cluster (see ossKarpenterAzureVersion/ossKarpenterAzureCommit), via its exported " +
		"bootstrap.AKS{}.Script(). This detects compatibility breaks between scripts baked into the " +
		"AgentBaker VHD and Karpenter's independently maintained CSE template, without building, running, " +
		"or patching the Karpenter controller.",
	Config: Config{
		Cluster:            ClusterOSSKarpenter,
		VHD:                config.VHDUbuntu2204Gen2Containerd,
		CustomDataOverride: renderOSSKarpenterCustomData,
	},
})

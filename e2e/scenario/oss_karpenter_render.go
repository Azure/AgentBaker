package scenario

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/karpenter-provider-azure/pkg/providers/imagefamily/bootstrap"
)

// renderOSSKarpenterCustomData renders the exact node CustomData the pinned
// upstream karpenter-provider-azure controller (ossKarpenterAzureVersion /
// ossKarpenterAzureCommit) would generate for this scenario's cluster and
// VHD, by calling the same exported entry point the controller calls
// internally:
//
//	imagefamily.Ubuntu2204.ScriptlessCustomData(...) -> bootstrap.AKS{}.Script()
//
// No source patch, controller build, or CRDs are involved: this is a direct
// library call against github.com/Azure/agentbaker/e2e's pinned dependency on
// karpenter-provider-azure (see e2e/go.mod, pinned to the same commit as
// ossKarpenterAzureCommit). The rendered script replaces AgentBaker's own CSE
// via Config.CustomDataOverride, so the rest of the scenario (VMSS creation,
// waiting for Ready, default pod-scheduling validation) is the normal
// AgentBaker E2E node lifecycle - only the CSE content differs.
//
// Every input mirrors a field AgentBaker's own NBC rendering already derives
// from the same cluster (see node_config.go), so no extra Azure lookups are
// needed: NSG/route table names use the identical aks-agentpool-<clusterid>-*
// naming convention as karpenter-provider-azure's own launchtemplate.go.
func renderOSSKarpenterCustomData(_ context.Context, s *Scenario) (customData string, cseCmd string, err error) {
	if s.Runtime == nil || s.Runtime.NBC == nil || s.Runtime.NBC.ContainerService == nil ||
		s.Runtime.NBC.ContainerService.Properties == nil || s.Runtime.Cluster == nil ||
		s.Runtime.Cluster.Model == nil || s.Runtime.Cluster.Model.Properties == nil ||
		s.Runtime.Cluster.Model.Name == nil || s.Runtime.Cluster.Model.Location == nil ||
		s.Runtime.Cluster.Model.Properties.NodeResourceGroup == nil ||
		s.Runtime.Cluster.ClusterParams == nil || s.Runtime.Cluster.SubnetID == "" ||
		s.Runtime.Kube == nil || s.Runtime.Kube.RESTConfig == nil || s.VHD == nil {
		return "", "", fmt.Errorf("scenario runtime is incomplete for OSS Karpenter CSE render")
	}

	cluster := s.Runtime.Cluster
	props := s.Runtime.NBC.ContainerService.Properties
	network := cluster.Model.Properties.NetworkProfile
	if network == nil || network.NetworkPlugin == nil {
		return "", "", fmt.Errorf("cluster network profile is incomplete for OSS Karpenter CSE render")
	}
	networkPolicy := ""
	if network.NetworkPolicy != nil {
		networkPolicy = string(*network.NetworkPolicy)
	}
	if props.OrchestratorProfile == nil || props.OrchestratorProfile.OrchestratorVersion == "" {
		return "", "", fmt.Errorf("cluster kubernetes version is unavailable for OSS Karpenter CSE render")
	}
	if len(cluster.ClusterParams.CACert) == 0 || cluster.ClusterParams.BootstrapToken == "" || cluster.ClusterParams.FQDN == "" {
		return "", "", fmt.Errorf("cluster credentials are unavailable for OSS Karpenter CSE render")
	}

	// bootstrap.AKS dereferences CABundle directly (aksbootstrap.go: nbv.KubeCACrt
	// = *a.CABundle) and expects it pre-base64-encoded, matching how AgentBaker's
	// own NBC forwards the same cert downstream (see node_config.go's
	// KubernetesCaCert assembly).
	caBundle := base64.StdEncoding.EncodeToString(cluster.ClusterParams.CACert)

	aks := bootstrap.AKS{
		Options: bootstrap.Options{
			ClusterName:     *cluster.Model.Name,
			ClusterEndpoint: s.Runtime.Kube.RESTConfig.Host,
			CABundle:        &caBundle,
			SubnetID:        cluster.SubnetID,
		},
		Arch:                           s.VHD.Arch,
		TenantID:                       cluster.TenantID,
		SubscriptionID:                 config.Config.SubscriptionID,
		Location:                       *cluster.Model.Location,
		KubeletIdentityClientID:        s.Runtime.NBC.UserAssignedIdentityClientID,
		ResourceGroup:                  *cluster.Model.Properties.NodeResourceGroup,
		NetworkSecurityGroupName:       props.GetNSGName(),
		RouteTableName:                 props.GetRouteTableName(),
		APIServerName:                  cluster.ClusterParams.FQDN,
		KubeletClientTLSBootstrapToken: cluster.ClusterParams.BootstrapToken,
		NetworkPlugin:                  string(*network.NetworkPlugin),
		NetworkPolicy:                  networkPolicy,
		KubernetesVersion:              props.OrchestratorProfile.OrchestratorVersion,
	}

	rendered, err := aks.Script()
	if err != nil {
		return "", "", fmt.Errorf("render OSS Karpenter CSE: %w", err)
	}
	// Karpenter's aksscriptless mode (the mode this mirrors) uses no separate
	// CSE VM extension: the rendered script is the entire CustomData payload,
	// same as AgentBaker's own scriptless mode.
	return rendered, "", nil
}

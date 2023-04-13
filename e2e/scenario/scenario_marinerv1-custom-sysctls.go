package scenario

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

func marinerv1CustomSysctls() *Scenario {
	return &Scenario{
		Name:        "marinerv1-custom-sysctls",
		Description: "tests that a marinerV1 VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			BootstrapConfigMutator: func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration) {
				customLinuxConfig := &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr[int32](300000),
						NetNetfilterNfConntrackBuckets: to.Ptr[int32](147456),
					},
				}
				nbc.AgentPoolProfile.CustomLinuxOSConfig = customLinuxConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = customLinuxConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v1"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v1"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
					ID: to.Ptr(DefaultImageVersionIDs["marinerv1"]),
				}
			},
			LiveVMValidators: []*LiveVMValidator{
				{
					Description: "assert custom sysctl settings",
					Command:     "sysctl -a",
					Asserter: func(stdout, stderr string) error {
						if !strings.Contains(stdout, "net.netfilter.nf_conntrack_buckets = 147456") {
							return fmt.Errorf("expected to find net.netfilter.nf_conntrack_buckets set to 147456, but was not")
						}
						if !strings.Contains(stdout, "net.netfilter.nf_conntrack_max = 300000") {
							return fmt.Errorf("expected to find net.netfilter.nf_conntrack_max set to 300000, but was not")
						}
						return nil
					},
				},
			},
		},
	}
}

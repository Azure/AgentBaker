package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
)

func marinerv2CustomSysctls() *Scenario {
	customIntSysctls := map[string]int{
		"net.netfilter.nf_conntrack_max":     2097152,
		"net.netfilter.nf_conntrack_buckets": 524288,
		"net.ipv4.tcp_keepalive_intvl":       90,
	}
	customStringSysctls := map[string]string{
		"net.ipv4.ip_local_port_range": "32768 65535",
	}
	return &Scenario{
		Name:        "marinerv2-custom-sysctls",
		Description: "tests that a MarinerV2 VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				customLinuxConfig := &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr(int32(customIntSysctls["net.netfilter.nf_conntrack_max"])),
						NetNetfilterNfConntrackBuckets: to.Ptr(int32(customIntSysctls["net.netfilter.nf_conntrack_buckets"])),
						NetIpv4IpLocalPortRange:        customStringSysctls["net.ipv4.ip_local_port_range"],
						NetIpv4TcpkeepaliveIntvl:       to.Ptr(int32(customIntSysctls["net.ipv4.tcp_keepalive_intvl"])),
					},
				}
				nbc.AgentPoolProfile.CustomLinuxOSConfig = customLinuxConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = customLinuxConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = "aks-cblmariner-v2-gen2"
				nbc.AgentPoolProfile.Distro = "aks-cblmariner-v2-gen2"
			},
			VMConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
				vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{
					ID: to.Ptr(DefaultImageVersionIDs["marinerv2"]),
				}
			},
			LiveVMValidators: []*LiveVMValidator{
				SysctlConfigValidator(customIntSysctls),
				SysctlConfigValidator(customStringSysctls),
			},
		},
	}
}

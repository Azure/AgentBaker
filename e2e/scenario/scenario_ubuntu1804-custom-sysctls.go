package scenario

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
)

func ubuntu1804CustomSysctls() *Scenario {
	customSysctls := map[string]int{
		"net.netfilter.nf_conntrack_max":     200000,
		"net.netfilter.nf_conntrack_buckets": 75264,
		"net.ipv4.tcp_keepalive_intvl":       10,
	}
	return &Scenario{
		Name:        "ubuntu1804-custom-sysctls",
		Description: "tests that an ubuntu 1804 VHD can be properly bootstrapped when supplied custom node config that contains custom sysctl settings",
		Config: Config{
			ClusterSelector: NetworkPluginKubenetSelector,
			ClusterMutator:  NetworkPluginKubenetMutator,
			BootstrapConfigMutator: func(nbc *datamodel.NodeBootstrappingConfiguration) {
				customLinuxConfig := &datamodel.CustomLinuxOSConfig{
					Sysctls: &datamodel.SysctlConfig{
						NetNetfilterNfConntrackMax:     to.Ptr(int32(customSysctls["net.netfilter.nf_conntrack_max"])),
						NetNetfilterNfConntrackBuckets: to.Ptr(int32(customSysctls["net.netfilter.nf_conntrack_buckets"])),
						NetIpv4TcpkeepaliveIntvl:       to.Ptr(int32(customSysctls["net.ipv4.tcp_keepalive_intvl"])),
					},
				}
				nbc.AgentPoolProfile.CustomLinuxOSConfig = customLinuxConfig
				nbc.ContainerService.Properties.AgentPoolProfiles[0].CustomLinuxOSConfig = customLinuxConfig
			},
			LiveVMValidators: []*LiveVMValidator{
				SysctlConfigValidator(customSysctls),
			},
		},
	}
}

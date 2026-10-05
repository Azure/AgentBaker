package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/stretchr/testify/require"
)

func TestGenerateLocalDNSCoreFileLoadBalance(t *testing.T) {
	for _, includeHosts := range []bool{false, true} {
		t.Run(fmt.Sprintf("hosts=%t", includeHosts), func(t *testing.T) {
			config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{
				EnableLocalDNS:   true,
				VnetDNSOverrides: map[string]*datamodel.LocalDNSOverrides{},
				KubeDNSOverrides: map[string]*datamodel.LocalDNSOverrides{},
			}
			for _, overrides := range []map[string]*datamodel.LocalDNSOverrides{
				config.AgentPoolProfile.LocalDNSProfile.VnetDNSOverrides,
				config.AgentPoolProfile.LocalDNSProfile.KubeDNSOverrides,
			} {
				for _, zone := range []string{".", "cluster.local", "example.test"} {
					overrides[zone] = &datamodel.LocalDNSOverrides{
						ForwardDestination: "ClusterCoreDNS", ForwardPolicy: "Sequential",
						MaxConcurrent: to.Int32Ptr(1000), CacheDurationInSeconds: to.Int32Ptr(300),
						ServeStale: "Disable",
					}
				}
			}
			corefile, err := GenerateLocalDNSCoreFile(config, config.AgentPoolProfile, includeHosts)
			require.NoError(t, err)
			// Check each server block, including custom zones, rather than only the root block.
			for _, block := range strings.Split(corefile, "\n}\n") {
				if strings.Contains(block, "\n    forward . ") {
					require.Equal(t, 1, strings.Count(block, "\n    loadbalance\n"), block)
					require.Contains(t, block, "policy sequential")
				} else {
					require.NotContains(t, block, "loadbalance", "health-check block must stay unchanged")
				}
			}
			require.Equal(t, 6, strings.Count(corefile, "\n    loadbalance\n"))
		})
	}
}

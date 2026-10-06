package scenario

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/stretchr/testify/require"
)

func TestLocalDNSForwardsToClusterDNSServiceIP(t *testing.T) {
	assertCoreDNSForwarding := func(t *testing.T, corefile string) {
		t.Helper()
		require.Contains(t, corefile, "forward . "+clusterDNSServiceIP)
		require.NotContains(t, corefile, "10.0.0.10", "LocalDNS must not use the default service IP")
	}

	t.Run("CSE", func(t *testing.T) {
		nbc, err := baseTemplateLinux("eastus", "1.34.0", "amd64")
		require.NoError(t, err)

		for _, variant := range []struct {
			name         string
			includeHosts bool
		}{
			{name: "base"},
			{name: "with_hosts", includeHosts: true},
		} {
			t.Run(variant.name, func(t *testing.T) {
				// CSE uses this separate top-level profile, not the nested AgentPoolProfiles[0].
				corefile, err := agent.GenerateLocalDNSCoreFile(nbc, nbc.AgentPoolProfile, variant.includeHosts)
				require.NoError(t, err)
				assertCoreDNSForwarding(t, corefile)
			})
		}
	})

	t.Run("ANC", func(t *testing.T) {
		nbc, err := baseTemplateLinux("eastus", "1.34.0", "amd64")
		require.NoError(t, err)
		cfg, err := nbcToAKSNodeConfigV1(nbc)
		require.NoError(t, err)
		require.Equal(t, clusterDNSServiceIP, cfg.GetClusterConfig().GetClusterNetworkConfig().GetCoreDnsServiceIp())

		// Build the command to inspect its rendered corefiles without executing provisioning.
		cmd, err := parser.BuildCSECmd(t.Context(), cfg, nil)
		require.NoError(t, err)
		env := make(map[string]string)
		for _, entry := range cmd.Env {
			key, value, _ := strings.Cut(entry, "=")
			env[key] = value
		}
		for _, key := range []string{"LOCALDNS_GENERATED_COREFILE", "LOCALDNS_COREFILE_BASE", "LOCALDNS_COREFILE_WITH_HOSTS"} {
			t.Run(key, func(t *testing.T) {
				require.NotEmpty(t, env[key])
				corefile, err := base64.StdEncoding.DecodeString(env[key])
				require.NoError(t, err)
				assertCoreDNSForwarding(t, string(corefile))
			})
		}
	})
}

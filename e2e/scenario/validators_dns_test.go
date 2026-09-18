package scenario

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestServiceDNSAnswer(t *testing.T) {
	for _, tc := range []struct {
		name, answer string
		pass         bool
	}{
		{"service", "172.16.0.1\n", true},
		{"default-subnet", "10.0.0.1\n", false},
		{"NXDOMAIN-empty", "", false},
		{"SERVFAIL", ";; communications error: timed out", false},
		{"extra-answer", "172.16.0.1\n10.0.0.1\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.pass, validateServiceDNSAnswer(tc.answer, "172.16.0.1") == nil)
		})
	}
}

func TestDNSWorkloadRejectsBrokenResolver(t *testing.T) {
	for _, tc := range []struct {
		name, nameserver, answer, exit string
		pass                           bool
	}{
		{"localdns", "169.254.10.11", "Name: kubernetes.default.svc.cluster.local\nAddress: 172.16.0.1", "0", true},
		{"busybox-numbered", "169.254.10.11", "Name: kubernetes.default.svc.cluster.local\nAddress 1: 172.16.0.1", "0", true},
		{"bypasses-localdns", "172.16.0.53", "Name: kubernetes.default.svc.cluster.local\nAddress: 172.16.0.1", "0", false},
		{"wrong-answer", "169.254.10.11", "Name: kubernetes.default.svc.cluster.local\nAddress: 10.0.0.1", "0", false},
		{"server-is-not-answer", "169.254.10.11", "Server: 172.16.0.1\nAddress: 172.16.0.1", "0", false},
		{"NXDOMAIN", "169.254.10.11", "NXDOMAIN", "1", false},
		{"extra-resolver", "169.254.10.11\nnameserver 168.63.129.16", "Name: kubernetes.default.svc.cluster.local\nAddress: 172.16.0.1", "0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			resolv := filepath.Join(dir, "resolv.conf")
			require.NoError(t, os.WriteFile(resolv, []byte("nameserver "+tc.nameserver+"\n"), 0600))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "nslookup"), []byte("#!/bin/sh\nprintf '%s\\n' \"$DNS_ANSWER\"\nexit \"$DNS_EXIT\"\n"), 0700))
			script, err := dnsWorkloadScript("169.254.10.11", "172.16.0.1")
			require.NoError(t, err)
			cmd := exec.Command("sh", "-c", strings.ReplaceAll(script, "/etc/resolv.conf", resolv))
			cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "DNS_ANSWER="+tc.answer, "DNS_EXIT="+tc.exit)
			out, err := cmd.CombinedOutput()
			require.Equal(t, tc.pass, err == nil, "%s", out)
		})
	}
}

// This exercises the AB fixture converter and real ANC parser. The RP PR
// independently covers its production converters and VM CustomData serialization.
func TestDNSFixtureSurvivesANCSerialization(t *testing.T) {
	nbc, err := baseTemplateLinux("eastus", "1.34.1", "amd64")
	require.NoError(t, err)
	nbc.AgentPoolProfile.KubernetesConfig.DNSServiceIP = "172.16.0.53"
	nbc.SecureTLSBootstrappingConfig = &datamodel.SecureTLSBootstrappingConfig{}
	config, err := nbcToAKSNodeConfigV1(nbc)
	require.NoError(t, err)
	raw, err := nodeconfigutils.MarshalConfigurationV1(config)
	require.NoError(t, err)
	decoded, err := nodeconfigutils.UnmarshalConfigurationV1(raw)
	require.NoError(t, err)
	require.Equal(t, "172.16.0.53", decoded.GetClusterConfig().GetClusterNetworkConfig().GetCoreDnsServiceIp())
	cmd, err := parser.BuildCSECmd(t.Context(), decoded, nil)
	require.NoError(t, err)
	found := 0
	for _, env := range cmd.Env {
		key, value, _ := strings.Cut(env, "=")
		if key == "LOCALDNS_GENERATED_COREFILE" || key == "LOCALDNS_COREFILE_BASE" || key == "LOCALDNS_COREFILE_WITH_HOSTS" {
			corefile, err := base64.StdEncoding.DecodeString(value)
			require.NoError(t, err)
			require.Contains(t, string(corefile), "bind 169.254.10.10")
			require.Contains(t, string(corefile), "bind 169.254.10.11")
			require.Contains(t, string(corefile), "forward . 172.16.0.53")
			require.NotContains(t, string(corefile), "forward . 10.0.0.10")
			found++
		}
	}
	require.Equal(t, 3, found, "all ANC corefile variants must preserve the requested upstream")
}

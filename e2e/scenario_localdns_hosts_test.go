package e2e

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	desiredLocalDNSVersion          = "e2e-localdns-corefile-version"
	localDNSPayloadPath             = "/opt/azure/containers/localdns/e2e-localdns-lps-payload.json"
	localDNSFetcherStamp            = "/opt/azure/containers/localdns/e2e-localdns-lps-fetcher-called"
	localDNSBranchScriptArchivePath = "/opt/azure/containers/localdns/e2e-localdns.sh.gz.b64"
	localDNSFetcherPath             = "/opt/azure/containers/localdns/e2e-fetch-localdns-config"
)

// Test_LocalDNSHostsPlugin tests the localdns hosts plugin across all supported distros
// on the legacy (bash CSE) bootstrap path.
// Hosts plugin validators (IP match, Corefile, hosts file) run automatically
// via ValidateCommonLinux when EnableHostsPlugin is set; this test does not assert
// on DNS flags such as AA/RA.
//
// Run a single distro with: go test -run "Test_LocalDNSHostsPlugin/AzureLinuxV3" -v
func Test_LocalDNSHostsPlugin(t *testing.T) {
	tests := []struct {
		name            string
		vhd             *config.Image
		vmConfigMutator func(*armcompute.VirtualMachineScaleSet)
	}{
		{name: "Ubuntu2204", vhd: config.VHDUbuntu2204Gen2Containerd},
		{name: "Ubuntu2404", vhd: config.VHDUbuntu2404Gen2Containerd},
		{name: "Ubuntu2604Minimal", vhd: config.VHDUbuntu2604MinimalGen2Containerd},
		{name: "AzureLinuxV3", vhd: config.VHDAzureLinuxV3Gen2},
		{name: "ACL", vhd: config.VHDACLGen2TL, vmConfigMutator: func(vmss *armcompute.VirtualMachineScaleSet) {
			vmss.Properties = addTrustedLaunchToVMSS(vmss.Properties)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cluster := ClusterKubenet
			if tt.name == "Ubuntu2604Minimal" {
				cluster = ClusterLatestKubernetesVersionKubenet
			}
			RunScenario(t, &Scenario{
				Description: "Tests that localdns hosts plugin works correctly on " + tt.name,
				Config: Config{
					Cluster: cluster,
					VHD:     tt.vhd,
					BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.LocalDNSProfile.EnableHostsPlugin = true
						nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
					},
					AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
						config.LocalDnsProfile.EnableHostsPlugin = true
						config.LocalDnsProfile.EnableLocalDns = true
					},
					VMConfigMutator: tt.vmConfigMutator,
				},
			})
		})
	}
}

// Test_LocalDNSLPSBootstrapPatch validates the node-side LocalDNS live-patching
// bootstrap path. It simulates LPS by temporarily wrapping aks-node-controller's
// fetch-localdns-config command so it returns a LocalDNS nodeConfig payload, then
// delegates to the real apply-localdns-config implementation. The test verifies that
// localdns.sh:
//  1. invokes the fetcher before CoreDNS starts,
//  2. renders the supplied LocalDNS profile payload into updated.localdns.corefile,
//  3. persists the paired corefileVersion, and
//  4. stamps components.localDNS.current after kubeconfig/node registration.
func Test_LocalDNSLPSBootstrapPatch(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests LocalDNS LPS bootstrap patching applies Corefile and reports corefileVersion",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			// Force compiling the local aks-node-controller so the provision-config parser matches
			// the baker-generated nbc-cmd for Corefile env vars (compareEnvs parity).
			ForceScriptlessCompilation: true,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
			},
			CustomDataWriteFiles: []CustomDataWriteFile{
				{
					Path:        localDNSBranchScriptArchivePath,
					Permissions: "0644",
					Owner:       "root",
					Content:     mustReadCompressedLocalDNSArtifact(t),
				},
				{
					Path:        "/etc/systemd/system/localdns.service.d/00-e2e-branch-localdns.conf",
					Permissions: "0644",
					Owner:       "root",
					Content:     localDNSBranchScriptDropIn(),
				},
				{
					Path:        localDNSPayloadPath,
					Permissions: "0644",
					Owner:       "root",
					Content:     localDNSLPSPayload(desiredLocalDNSVersion),
				},
				{
					Path:        localDNSFetcherPath,
					Permissions: "0755",
					Owner:       "root",
					Content:     localDNSLPSFetcherWrapper(),
				},
			},
			AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
				config.LocalDnsProfile.EnableLocalDns = true
			},
			Validator: validateLocalDNSLPSBootstrapPatch,
		},
	})
}

func mustReadCompressedLocalDNSArtifact(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../parts/linux/cloud-init/artifacts/localdns.sh")
	require.NoError(t, err)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err = zw.Write(data)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	content := strings.ReplaceAll(string(data), `AKS_NODE_CONTROLLER_BINARY="/opt/azure/containers/aks-node-controller"`, `AKS_NODE_CONTROLLER_BINARY="`+localDNSFetcherPath+`"`)
	buf.Reset()
	zw = gzip.NewWriter(&buf)
	_, err = zw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func localDNSBranchScriptDropIn() string {
	return `[Service]
ExecStartPre=/bin/bash -c 'base64 -d ` + localDNSBranchScriptArchivePath + ` | gzip -d > /opt/azure/containers/localdns/localdns.sh && chmod 0544 /opt/azure/containers/localdns/localdns.sh'
`
}

func localDNSLPSPayload(version string) string {
	return `{
  "agentPools": {
    "nodepool2": {
      "corefileVersion": "` + version + `",
      "localDnsProfile": {
        "enableLocalDns": true,
        "vnetDnsOverrides": {
          ".": {
            "queryLogging": "Error",
            "protocol": "PreferUDP",
            "forwardDestination": "VnetDNS",
            "forwardPolicy": "Sequential",
            "maxConcurrent": 1000,
            "cacheDurationInSeconds": 3600,
            "serveStaleDurationInSeconds": 3600,
            "serveStale": "Immediate"
          }
        },
        "kubeDnsOverrides": {
          "cluster.local": {
            "queryLogging": "Error",
            "protocol": "PreferUDP",
            "forwardDestination": "ClusterCoreDNS",
            "forwardPolicy": "Sequential",
            "maxConcurrent": 1000,
            "cacheDurationInSeconds": 3600,
            "serveStaleDurationInSeconds": 3600,
            "serveStale": "Immediate"
          }
        }
      }
    }
  }
}`
}

func localDNSLPSFetcherWrapper() string {
	return `#!/bin/bash
set -euo pipefail
if [ "${1:-}" != "fetch-localdns-config" ]; then
    exec /opt/azure/containers/aks-node-controller "$@"
fi
output=""
while [ "$#" -gt 0 ]; do
    case "$1" in
        --output)
            output="$2"
            shift 2
            ;;
        *)
            shift
            ;;
    esac
done
if [ -z "$output" ]; then
    echo "missing --output" >&2
    exit 1
fi
touch ` + localDNSFetcherStamp + `
exec /opt/azure/containers/aks-node-controller apply-localdns-config --config-file ` + localDNSPayloadPath + ` --output "$output"
`
}

func validateLocalDNSLPSBootstrapPatch(ctx context.Context, s *Scenario) {
	const (
		updatedCorefile     = "/opt/azure/containers/localdns/updated.localdns.corefile"
		livepatchedCorefile = "/opt/azure/containers/localdns/livepatched.localdns.corefile"
	)

	ValidateFileExists(ctx, s, localDNSFetcherStamp)
	ValidateFileHasContent(ctx, s, updatedCorefile, "health-check.localdns.local:53")
	ValidateFileHasContent(ctx, s, updatedCorefile, "cluster.local:53")
	ValidateFileHasContent(ctx, s, livepatchedCorefile+".version", desiredLocalDNSVersion)
	ValidateLocalDNSService(ctx, s, "enabled")
	ValidateLocalDNSResolution(ctx, s, "169.254.10.10")

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		status := node.Annotations["kubernetes.azure.com/live-patching-status"]
		return strings.Contains(status, `"localDNS":{"current":"`+desiredLocalDNSVersion+`"}`), nil
	})
	require.NoError(s.T, err, "node did not report LocalDNS live-patching current version %q", desiredLocalDNSVersion)
}

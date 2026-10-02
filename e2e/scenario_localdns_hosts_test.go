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
	localDNSUnavailableFetcherStamp = "/opt/azure/containers/localdns/e2e-localdns-lps-unavailable-called"
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

// Test_LocalDNSLPSUnavailableFallback validates the unhappy bootstrap path: when LPS
// has no LocalDNS config published for the node (fetch-localdns-config fails open with
// no livepatched Corefile written), localdns.sh must fall back to the baked/CSE-generated
// localdns.corefile and CoreDNS must still come up and serve DNS. This is the failure-mode
// counterpart to Test_LocalDNSLPSBootstrapPatch, exercised end-to-end on a live node.
//
// The test verifies that:
//  1. the fetcher was invoked (LPS was consulted),
//  2. no livepatched Corefile or version file is written,
//  3. updated.localdns.corefile is still produced (from the baked source),
//  4. localdns.service is enabled and resolves DNS, and
//  5. the node does NOT report a LocalDNS live-patching current version.
func Test_LocalDNSLPSUnavailableFallback(t *testing.T) {
	RunScenario(t, &Scenario{
		Description: "Tests LocalDNS bootstrap falls back to the baked Corefile when LPS has no config",
		Config: Config{
			Cluster: ClusterKubenet,
			VHD:     config.VHDUbuntu2404Gen2Containerd,
			// Force compiling the local aks-node-controller so localdns.sh under test matches the
			// branch's Corefile generation (compareEnvs parity), same as the happy-path scenario.
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
					Path:        localDNSFetcherPath,
					Permissions: "0755",
					Owner:       "root",
					Content:     localDNSLPSUnavailableFetcherWrapper(),
				},
			},
			AKSNodeConfigMutator: func(_ *Cluster, config *aksnodeconfigv1.Configuration) {
				config.LocalDnsProfile.EnableLocalDns = true
			},
			Validator: validateLocalDNSLPSUnavailableFallback,
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
	// LOCALDNS_ENABLE_LEGACY_LIVEPATCH_STATUS makes localdns.sh write the
	// live-patching-status node annotation itself. In production the knead
	// live-patching loop owns this annotation, but knead does not drive the
	// localDNS component in this E2E, so opt into the bootstrap writer here.
	return `[Service]
Environment="LOCALDNS_ENABLE_LEGACY_LIVEPATCH_STATUS=true"
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
# Prefer the locally-compiled aks-node-controller (delivered via the hotfix path by
# ForceScriptlessCompilation) so apply-localdns-config matches the branch under test;
# fall back to the VHD baked-in binary otherwise.
ANC_BIN=/opt/azure/containers/aks-node-controller
if [ -x /opt/azure/containers/aks-node-controller-hotfix ]; then
    ANC_BIN=/opt/azure/containers/aks-node-controller-hotfix
fi
if [ "${1:-}" != "fetch-localdns-config" ]; then
    exec "$ANC_BIN" "$@"
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
exec "$ANC_BIN" apply-localdns-config --config-file ` + localDNSPayloadPath + ` --output "$output"
`
}

// localDNSLPSUnavailableFetcherWrapper simulates LPS having no LocalDNS config for the node.
// It records that fetch-localdns-config was invoked, then exits 0 without writing the output
// Corefile -- the same fail-open behavior aks-node-controller exhibits when LPS returns a
// benign "unavailable" status (NotFound/PermissionDenied/Unauthenticated). Non-fetch
// subcommands are delegated to the real binary so the rest of provisioning is unaffected.
func localDNSLPSUnavailableFetcherWrapper() string {
	return `#!/bin/bash
set -euo pipefail
ANC_BIN=/opt/azure/containers/aks-node-controller
if [ -x /opt/azure/containers/aks-node-controller-hotfix ]; then
    ANC_BIN=/opt/azure/containers/aks-node-controller-hotfix
fi
if [ "${1:-}" != "fetch-localdns-config" ]; then
    exec "$ANC_BIN" "$@"
fi
# LPS has nothing published for this node: record the call and fail open without writing a
# livepatched Corefile, so localdns.sh falls back to the baked localdns.corefile.
touch ` + localDNSUnavailableFetcherStamp + `
exit 0
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

func validateLocalDNSLPSUnavailableFallback(ctx context.Context, s *Scenario) {
	const (
		bakedCorefile       = "/opt/azure/containers/localdns/localdns.corefile"
		updatedCorefile     = "/opt/azure/containers/localdns/updated.localdns.corefile"
		livepatchedCorefile = "/opt/azure/containers/localdns/livepatched.localdns.corefile"
	)

	// The fetcher ran (LPS was consulted) but wrote nothing, so no livepatched Corefile
	// or version file should exist and localdns.sh must fall back to the baked Corefile.
	ValidateFileExists(ctx, s, localDNSUnavailableFetcherStamp)
	ValidateFileDoesNotExist(ctx, s, livepatchedCorefile)
	ValidateFileDoesNotExist(ctx, s, livepatchedCorefile+".version")

	// CoreDNS still comes up from the baked source and serves DNS.
	ValidateFileExists(ctx, s, bakedCorefile)
	ValidateFileHasContent(ctx, s, updatedCorefile, "health-check.localdns.local:53")
	ValidateLocalDNSService(ctx, s, "enabled")
	ValidateLocalDNSResolution(ctx, s, "169.254.10.10")

	// No LocalDNS live-patching version should be reported when LPS had nothing to apply.
	node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
	require.NoError(s.T, err, "failed to get node %s", s.Runtime.VM.KubeName)
	status := node.Annotations["kubernetes.azure.com/live-patching-status"]
	require.NotContains(s.T, status, `"localDNS"`,
		"node unexpectedly reported a LocalDNS live-patching status when LPS had no config: %q", status)
}

package scenario

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
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

	localDNSBakedCorefile       = "/opt/azure/containers/localdns/localdns.corefile"
	localDNSUpdatedCorefile     = "/opt/azure/containers/localdns/updated.localdns.corefile"
	localDNSLivepatchedCorefile = "/opt/azure/containers/localdns/livepatched.localdns.corefile"
)

// LocalDNSLPSBootstrapPatch validates the node-side LocalDNS live-patching bootstrap path.
// It simulates LPS by temporarily wrapping aks-node-controller's fetch-localdns-config command
// so it returns a LocalDNS nodeConfig payload, then delegates to the real apply-localdns-config
// implementation. The scenario verifies that localdns.sh:
//  1. invokes the fetcher before CoreDNS starts,
//  2. renders the supplied LocalDNS profile payload into updated.localdns.corefile,
//  3. persists the paired corefileVersion, and
//  4. stamps components.localDNS.current after kubeconfig/node registration.
var _ = Register(&Scenario{
	Name:        "LocalDNSLPSBootstrapPatch",
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
				Content:     mustReadCompressedLocalDNSArtifact(),
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

// LocalDNSLPSUnavailableFallback validates the unhappy bootstrap path: when LPS has no
// LocalDNS config published for the node (fetch-localdns-config fails open with no
// livepatched Corefile written), localdns.sh must fall back to the baked/CSE-generated
// localdns.corefile and CoreDNS must still come up and serve DNS. This is the failure-mode
// counterpart to LocalDNSLPSBootstrapPatch, exercised end-to-end on a live node.
//
// The scenario verifies that:
//  1. the fetcher was invoked (LPS was consulted),
//  2. no livepatched Corefile or version file is written,
//  3. updated.localdns.corefile is still produced (from the baked source),
//  4. localdns.service is enabled and resolves DNS, and
//  5. the node does NOT report a LocalDNS live-patching current version.
var _ = Register(&Scenario{
	Name:        "LocalDNSLPSUnavailableFallback",
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
				Content:     mustReadCompressedLocalDNSArtifact(),
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

// mustReadCompressedLocalDNSArtifact returns the branch's localdns.sh, retargeted at the e2e
// fetcher wrapper, gzipped and base64-encoded for delivery through cloud-init write_files.
func mustReadCompressedLocalDNSArtifact() string {
	data, err := os.ReadFile(repoPath("parts/linux/cloud-init/artifacts/localdns.sh"))
	if err != nil {
		panic(fmt.Sprintf("read localdns.sh: %v", err))
	}
	content := strings.ReplaceAll(string(data), `AKS_NODE_CONTROLLER_BINARY="/opt/azure/containers/aks-node-controller"`, `AKS_NODE_CONTROLLER_BINARY="`+localDNSFetcherPath+`"`)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(content)); err != nil {
		panic(fmt.Sprintf("compress localdns.sh: %v", err))
	}
	if err := zw.Close(); err != nil {
		panic(fmt.Sprintf("compress localdns.sh: %v", err))
	}
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

func validateLocalDNSLPSBootstrapPatch(ctx context.Context, s *Scenario) error {
	if err := errors.Join(
		ValidateFileExists(ctx, s, localDNSFetcherStamp),
		ValidateFileHasContent(ctx, s, localDNSUpdatedCorefile, "health-check.localdns.local:53"),
		ValidateFileHasContent(ctx, s, localDNSUpdatedCorefile, "cluster.local:53"),
		ValidateFileHasContent(ctx, s, localDNSLivepatchedCorefile+".version", desiredLocalDNSVersion),
		ValidateLocalDNSService(ctx, s, "enabled"),
		ValidateLocalDNSResolution(ctx, s, "169.254.10.10"),
	); err != nil {
		return err
	}

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		status := node.Annotations["kubernetes.azure.com/live-patching-status"]
		return strings.Contains(status, `"localDNS":{"current":"`+desiredLocalDNSVersion+`"}`), nil
	})
	if err != nil {
		return fmt.Errorf("node did not report LocalDNS live-patching current version %q: %w", desiredLocalDNSVersion, err)
	}
	return nil
}

func validateLocalDNSLPSUnavailableFallback(ctx context.Context, s *Scenario) error {
	// The fetcher ran (LPS was consulted) but wrote nothing, so no livepatched Corefile
	// or version file should exist and localdns.sh must fall back to the baked Corefile.
	// CoreDNS still comes up from the baked source and serves DNS.
	if err := errors.Join(
		ValidateFileExists(ctx, s, localDNSUnavailableFetcherStamp),
		ValidateFileDoesNotExist(ctx, s, localDNSLivepatchedCorefile),
		ValidateFileDoesNotExist(ctx, s, localDNSLivepatchedCorefile+".version"),
		ValidateFileExists(ctx, s, localDNSBakedCorefile),
		ValidateFileHasContent(ctx, s, localDNSUpdatedCorefile, "health-check.localdns.local:53"),
		ValidateLocalDNSService(ctx, s, "enabled"),
		ValidateLocalDNSResolution(ctx, s, "169.254.10.10"),
	); err != nil {
		return err
	}

	// No LocalDNS live-patching version should be reported when LPS had nothing to apply.
	node, err := s.Runtime.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", s.Runtime.VM.KubeName, err)
	}
	status := node.Annotations["kubernetes.azure.com/live-patching-status"]
	if strings.Contains(status, `"localDNS"`) {
		return fmt.Errorf("node unexpectedly reported a LocalDNS live-patching status when LPS had no config: %q", status)
	}
	return nil
}

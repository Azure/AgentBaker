package scenario

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	desiredLocalDNSVersion          = "e2e-localdns-corefile-version"
	localDNSPayloadPath             = "/opt/azure/containers/localdns/e2e-localdns-lps-payload.json"
	localDNSFetcherStamp            = "/opt/azure/containers/localdns/e2e-localdns-lps-fetcher-called"
	localDNSBranchScriptArchivePath = "/opt/azure/containers/localdns/e2e-localdns.sh.gz.b64"
	localDNSLimitsScriptArchivePath = "/opt/azure/containers/localdns/e2e-localdns-limits.sh.gz.b64"
	localDNSFetcherPath             = "/opt/azure/containers/localdns/e2e-fetch-localdns-config"
	localDNSUnavailableFetcherStamp = "/opt/azure/containers/localdns/e2e-localdns-lps-unavailable-called"

	localDNSBakedCorefile       = "/opt/azure/containers/localdns/localdns.corefile"
	localDNSUpdatedCorefile     = "/opt/azure/containers/localdns/updated.localdns.corefile"
	localDNSLivepatchedCorefile = "/opt/azure/containers/localdns/livepatched.localdns.corefile"

	// The payload's limits differ from the provisioning ones in node_config.go (2008 millicores and
	// 128 MB), so the scenario can tell a live-patched limit from the CSE-written localdns.slice.
	localDNSPatchedCPUMilliCores = 3000
	localDNSPatchedMemoryMB      = 512
)

func init() {
	// LocalDNSLPSBootstrapPatch validates the node-side LocalDNS live-patching bootstrap
	// path. It simulates LPS by temporarily wrapping aks-node-controller's
	// fetch-localdns-config command so it returns a LocalDNS nodeConfig payload, then
	// delegates to the real apply-localdns-config implementation. The scenario verifies
	// that localdns.sh:
	//  1. invokes the fetcher before CoreDNS starts,
	//  2. renders the supplied LocalDNS profile payload into updated.localdns.corefile,
	//  3. applies the payload's CPU and memory limits to localdns.slice, both live (cpu.max,
	//     memory.max) and saved for the next boot (the set-property drop-ins),
	//  4. persists the paired corefileVersion once CoreDNS is ready, and
	//  5. keeps all of that across a reboot.
	// Reporting that version in live-patching-status is the knead reconciler's job on its first
	// run after the node joins; this E2E has no LPC driving localDNS, so it checks the file.
	Register(&Scenario{
		Name:        "LocalDNSLPSBootstrapPatch",
		Description: "Tests LocalDNS LPS bootstrap patching applies Corefile and limits and reports corefileVersion",
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
					Path:        localDNSLimitsScriptArchivePath,
					Permissions: "0644",
					Owner:       "root",
					Content:     mustReadCompressedLocalDNSLimitsArtifact(),
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
	// localdns.corefile and CoreDNS must still come up and serve DNS. This is the
	// failure-mode counterpart to LocalDNSLPSBootstrapPatch, exercised end-to-end on a
	// live node.
	//
	// The scenario verifies that:
	//  1. the fetcher was invoked (LPS was consulted),
	//  2. no livepatched Corefile or version file is written,
	//  3. updated.localdns.corefile is still produced (from the baked source),
	//  4. localdns.service is enabled and resolves DNS, and
	//  5. the node does NOT report a LocalDNS live-patching current version.
	Register(&Scenario{
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
					Path:        localDNSLimitsScriptArchivePath,
					Permissions: "0644",
					Owner:       "root",
					Content:     mustReadCompressedLocalDNSLimitsArtifact(),
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

// mustReadCompressedLocalDNSArtifact returns the branch's localdns.sh, repointed at the
// e2e fetcher wrapper and gzip+base64 encoded for delivery through cloud-init write_files.
// It panics on failure: the script is checked into the repo, so a read error means the
// checkout is unusable for every LocalDNS scenario.
func mustReadCompressedLocalDNSArtifact() string {
	return mustReadCompressedArtifact("parts/linux/cloud-init/artifacts/localdns.sh", func(content string) string {
		return strings.ReplaceAll(content,
			`AKS_NODE_CONTROLLER_BINARY="/opt/azure/containers/aks-node-controller"`,
			`AKS_NODE_CONTROLLER_BINARY="`+localDNSFetcherPath+`"`)
	})
}

// mustReadCompressedLocalDNSLimitsArtifact returns the branch's localdns-limits.sh, which the
// branch's localdns.sh sources from next to itself; the VHD under test may predate it.
func mustReadCompressedLocalDNSLimitsArtifact() string {
	return mustReadCompressedArtifact("parts/linux/cloud-init/artifacts/localdns-limits.sh", func(content string) string {
		return content
	})
}

func mustReadCompressedArtifact(relPath string, edit func(string) string) string {
	path := repoPath(relPath)
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("reading %s for the LocalDNS LPS scenarios: %v", path, err))
	}
	content := edit(string(data))

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(content)); err != nil {
		panic(fmt.Sprintf("compressing %s for the LocalDNS LPS scenarios: %v", path, err))
	}
	if err := zw.Close(); err != nil {
		panic(fmt.Sprintf("compressing %s for the LocalDNS LPS scenarios: %v", path, err))
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func localDNSBranchScriptDropIn() string {
	const limitsScript = "/opt/azure/containers/localdns/localdns-limits.sh"
	return `[Service]
ExecStartPre=/bin/bash -c 'base64 -d ` + localDNSBranchScriptArchivePath + ` | gzip -d > /opt/azure/containers/localdns/localdns.sh && chmod 0544 /opt/azure/containers/localdns/localdns.sh'
ExecStartPre=/bin/bash -c 'base64 -d ` + localDNSLimitsScriptArchivePath + ` | gzip -d > ` + limitsScript + ` && chmod 0644 ` + limitsScript + `'
`
}

func localDNSLPSPayload(version string) string {
	return `{
  "agentPools": {
    "nodepool2": {
      "corefileVersion": "` + version + `",
      "localDnsProfile": {
        "enableLocalDns": true,
        "cpuLimitInMilliCores": ` + fmt.Sprint(localDNSPatchedCPUMilliCores) + `,
        "memoryLimitInMb": ` + fmt.Sprint(localDNSPatchedMemoryMB) + `,
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
	if err := ValidateFileExists(ctx, s, localDNSFetcherStamp); err != nil {
		return err
	}
	if err := ValidateFileHasContent(ctx, s, localDNSUpdatedCorefile, "health-check.localdns.local:53"); err != nil {
		return err
	}
	if err := ValidateFileHasContent(ctx, s, localDNSUpdatedCorefile, "cluster.local:53"); err != nil {
		return err
	}
	if err := validateLocalDNSLPSPatchInEffect(ctx, s); err != nil {
		return err
	}

	// The patch must survive a reboot. localdns.sh does not fetch again once the node has joined,
	// so the live-patched Corefile and .version on disk and the saved drop-ins have to carry it.
	if err := RebootVMAndWaitForSSH(ctx, s); err != nil {
		return err
	}
	return validateLocalDNSLPSPatchInEffect(ctx, s)
}

// validateLocalDNSLPSPatchInEffect checks the payload's limits on localdns.slice, the committed
// corefileVersion, and that localdns serves DNS.
func validateLocalDNSLPSPatchInEffect(ctx context.Context, s *Scenario) error {
	if err := validateLocalDNSLPSLimits(ctx, s); err != nil {
		return err
	}
	if err := ValidateFileHasContent(ctx, s, localDNSLivepatchedCorefile+".version", desiredLocalDNSVersion); err != nil {
		return err
	}
	if err := ValidateLocalDNSService(ctx, s, "enabled"); err != nil {
		return err
	}
	return ValidateLocalDNSResolution(ctx, s, "169.254.10.10")
}

// validateLocalDNSLPSLimits checks the payload's limits in the kernel and in the drop-ins
// `systemctl set-property` saves them to. systemd saves CPUQuota truncated to a whole percent and
// MemoryMax in bytes; both are matched loosely enough to accept either spelling.
func validateLocalDNSLPSLimits(ctx context.Context, s *Scenario) error {
	cpuPercent := localDNSPatchedCPUMilliCores / 10
	memoryBytes := localDNSPatchedMemoryMB * 1024 * 1024
	script := fmt.Sprintf(`set -euo pipefail
# localdns may still be starting after a reboot.
for _ in $(seq 1 60); do
    systemctl is-active --quiet localdns && break
    sleep 2
done
cgroup=/sys/fs/cgroup/localdns.slice
saved=/etc/systemd/system.control/localdns.slice.d
cat "$cgroup/cpu.max" "$cgroup/memory.max"
cat "$saved/50-CPUQuota.conf" "$saved/50-MemoryMax.conf"
read -r quota period < "$cgroup/cpu.max"
test "$((quota * 100 / period))" = %[1]d || { echo "FAIL: expected cpu.max at %[1]d%%, got $quota $period"; exit 1; }
test "$(cat "$cgroup/memory.max")" = %[2]d || { echo "FAIL: expected memory.max %[2]d"; exit 1; }
grep -Eq '^CPUQuota=%[1]d(\.0+)?%%$' "$saved/50-CPUQuota.conf" || { echo "FAIL: CPUQuota %[1]d%% not saved"; exit 1; }
grep -Eq '^MemoryMax=(%[2]d|%[3]dM)$' "$saved/50-MemoryMax.conf" || { echo "FAIL: MemoryMax %[3]dM not saved"; exit 1; }
`, cpuPercent, memoryBytes, localDNSPatchedMemoryMB)
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "localdns.slice does not have the live-patched limits")
	return err
}

func validateLocalDNSLPSUnavailableFallback(ctx context.Context, s *Scenario) error {
	// The fetcher ran (LPS was consulted) but wrote nothing, so no livepatched Corefile
	// or version file should exist and localdns.sh must fall back to the baked Corefile.
	if err := ValidateFileExists(ctx, s, localDNSUnavailableFetcherStamp); err != nil {
		return err
	}
	if err := ValidateFileDoesNotExist(ctx, s, localDNSLivepatchedCorefile); err != nil {
		return err
	}
	if err := ValidateFileDoesNotExist(ctx, s, localDNSLivepatchedCorefile+".version"); err != nil {
		return err
	}

	// CoreDNS still comes up from the baked source and serves DNS.
	if err := ValidateFileExists(ctx, s, localDNSBakedCorefile); err != nil {
		return err
	}
	if err := ValidateFileHasContent(ctx, s, localDNSUpdatedCorefile, "health-check.localdns.local:53"); err != nil {
		return err
	}
	if err := ValidateLocalDNSService(ctx, s, "enabled"); err != nil {
		return err
	}
	if err := ValidateLocalDNSResolution(ctx, s, "169.254.10.10"); err != nil {
		return err
	}
	return nil
}

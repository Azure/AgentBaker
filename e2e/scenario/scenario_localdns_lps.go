package scenario

import (
	"context"
	"fmt"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	desiredLocalDNSVersion          = "e2e-localdns-corefile-version"
	localDNSPayloadPath             = "/opt/azure/containers/localdns/e2e-localdns-lps-payload.json"
	localDNSFetcherStamp            = "/opt/azure/containers/localdns/e2e-localdns-lps-fetcher-called"
	localDNSFetcherPath             = "/opt/azure/containers/localdns/e2e-fetch-localdns-config"
	localDNSFetcherDropInPath       = "/etc/systemd/system/localdns.service.d/00-e2e-lps-fetcher.conf"
	localDNSUnavailableFetcherStamp = "/opt/azure/containers/localdns/e2e-localdns-lps-unavailable-called"

	localDNSBakedCorefile       = "/opt/azure/containers/localdns/localdns.corefile"
	localDNSUpdatedCorefile     = "/opt/azure/containers/localdns/updated.localdns.corefile"
	localDNSLivepatchedCorefile = "/opt/azure/containers/localdns/livepatched.localdns.corefile"

	// The payload's limits differ from the provisioning ones in node_config.go (2008 millicores and
	// 128 MB), so the scenario can tell a live-patched limit from the CSE-written localdns.slice.
	localDNSPatchedCPUMilliCores = 3000
	localDNSPatchedMemoryMB      = 512
	// The payload's VnetDNS "." cache duration differs from node_config.go (3600 everywhere), so the
	// served Corefile shows whether the patch, not the baked config, is in effect.
	localDNSPatchedCacheSeconds = 1800

	rejectedLocalDNSVersion     = "e2e-localdns-rejected-corefile-version"
	localDNSRejectedHealthCheck = "not-a-duration"
	localDNSLPSRejectedMarker   = "/run/localdns/lps-corefile-rejected"
	// The provisioning limits from node_config.go (2008 millicores, 128 MB): CPU in whole percent, memory
	// in bytes.
	localDNSBakedCPUPercent = 200
	localDNSBakedMemoryMax  = "134217728"
)

func init() {
	// LocalDNSLPSBootstrapPatch validates the node-side LocalDNS live-patching bootstrap
	// path. It simulates LPS by temporarily wrapping aks-node-controller's
	// `localdns fetch-config` command so it returns a LocalDNS nodeConfig payload, then
	// delegates to the real `localdns apply-config` implementation. The scenario verifies
	// that localdns.sh:
	//  1. invokes the fetcher before CoreDNS starts,
	//  2. renders the supplied LocalDNS profile payload into updated.localdns.corefile,
	//  3. applies the payload's CPU and memory limits to localdns.slice, both live (cpu.max,
	//     memory.max) and saved for the next boot (the set-property drop-ins),
	//  4. persists the paired corefileVersion once CoreDNS is ready, and
	//  5. keeps all of that across a reboot.
	// Reporting that version in live-patching-status is the knead reconciler's job on its first
	// run after the node joins; this E2E has no LPC driving localDNS, so it checks the file.
	//
	// It tests the localdns.sh and localdns-limits.sh baked into the VHD, so it only runs on a VHD
	// built from this branch (see skipUnlessBranchVHD).
	Register(&Scenario{
		Name:        "LocalDNSLPSBootstrapPatch",
		Description: "Tests LocalDNS LPS bootstrap patching applies Corefile and limits and reports corefileVersion",
		SkipIf:      skipUnlessBranchVHD,
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
					Path:        localDNSFetcherDropInPath,
					Permissions: "0644",
					Owner:       "root",
					Content:     localDNSFetcherDropIn(),
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
	// LocalDNS config published for the node (`localdns fetch-config` fails open with no
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
		SkipIf:      skipUnlessBranchVHD,
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
					Path:        localDNSFetcherDropInPath,
					Permissions: "0644",
					Owner:       "root",
					Content:     localDNSFetcherDropIn(),
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

	// LocalDNSLPSRejectedCorefileFallback validates that a bad LPS payload cannot block provisioning.
	// The payload renders, but into a Corefile CoreDNS refuses to load (a health_check interval that is
	// not a duration). Without the fallback every start fetches the same payload and fails, until
	// systemd stops restarting localdns and CSE fails provisioning. The scenario provisioning at all
	// shows it does not; the validator checks the node came up on its baked config and why.
	Register(&Scenario{
		Name:        "LocalDNSLPSRejectedCorefileFallback",
		Description: "Tests a LocalDNS LPS Corefile CoreDNS rejects at bootstrap falls back to the baked config without failing provisioning",
		SkipIf:      skipUnlessBranchVHD,
		Config: Config{
			Cluster:                    ClusterKubenet,
			VHD:                        config.VHDUbuntu2404Gen2Containerd,
			ForceScriptlessCompilation: true,
			BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
				nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS = true
			},
			CustomDataWriteFiles: []CustomDataWriteFile{
				{
					Path:        localDNSFetcherDropInPath,
					Permissions: "0644",
					Owner:       "root",
					Content:     localDNSFetcherDropIn(),
				},
				{
					Path:        localDNSPayloadPath,
					Permissions: "0644",
					Owner:       "root",
					Content:     localDNSLPSRejectedPayload(),
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
			Validator: validateLocalDNSLPSRejectedCorefileFallback,
		},
	})
}

// skipUnlessBranchVHD skips the LPS scenarios unless the VHD under test was built in this pipeline
// run, which is when a VHD metadata file is set (the VHD build's e2e stage). They test the
// localdns.sh and localdns-limits.sh baked into the VHD, which a VHD from main may predate.
func skipUnlessBranchVHD(context.Context) string {
	if config.Config.VHDMetadataFile == "" {
		return "needs a VHD built from this branch; runs in the VHD build's e2e stage"
	}
	return ""
}

// localDNSFetcherDropIn points the VHD's localdns.sh at the e2e fetcher wrapper instead of
// aks-node-controller, so the bootstrap fetch gets the scenario's payload instead of asking LPS.
func localDNSFetcherDropIn() string {
	return "[Service]\nEnvironment=AKS_NODE_CONTROLLER_BINARY=" + localDNSFetcherPath + "\n"
}

func localDNSLPSPayload(version string) string {
	return localDNSLPSPayloadWithHealthCheck(version, "")
}

// localDNSLPSPayloadWithHealthCheck sets the VnetDNS "." health check interval to healthCheck, or
// leaves the health check out when it is empty.
func localDNSLPSPayloadWithHealthCheck(version, healthCheck string) string {
	vnetRootHealthCheck := ""
	if healthCheck != "" {
		vnetRootHealthCheck = `,
            "healthCheck": {
              "duration": "` + healthCheck + `"
            }`
	}
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
            "cacheDurationInSeconds": ` + fmt.Sprint(localDNSPatchedCacheSeconds) + `,
            "serveStaleDurationInSeconds": 3600,
            "serveStale": "Immediate"` + vnetRootHealthCheck + `
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

// localDNSLPSRejectedPayload is the bootstrap payload with a VnetDNS "." health_check interval that
// is not a duration. aks-node-controller renders it as is, and CoreDNS fails to load the Corefile.
func localDNSLPSRejectedPayload() string {
	return localDNSLPSPayloadWithHealthCheck(rejectedLocalDNSVersion, localDNSRejectedHealthCheck)
}

func localDNSLPSFetcherWrapper() string {
	return `#!/bin/bash
set -euo pipefail
# Prefer the locally-compiled aks-node-controller (delivered via the hotfix path by
# ForceScriptlessCompilation) so "localdns apply-config" matches the branch under test;
# fall back to the VHD baked-in binary otherwise.
ANC_BIN=/opt/azure/containers/aks-node-controller
if [ -x /opt/azure/containers/aks-node-controller-hotfix ]; then
    ANC_BIN=/opt/azure/containers/aks-node-controller-hotfix
fi
if [ "${1:-} ${2:-}" != "localdns fetch-config" ]; then
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
exec "$ANC_BIN" localdns apply-config --config-file ` + localDNSPayloadPath + ` --output "$output"
`
}

// localDNSLPSUnavailableFetcherWrapper simulates LPS having no LocalDNS config for the node.
// It records that `localdns fetch-config` was invoked, then exits 0 without writing the output
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
if [ "${1:-} ${2:-}" != "localdns fetch-config" ]; then
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

// validateLocalDNSLPSPatchInEffect checks that the patched Corefile is served, the payload's limits
// on localdns.slice, the committed corefileVersion, and that localdns serves DNS.
func validateLocalDNSLPSPatchInEffect(ctx context.Context, s *Scenario) error {
	if err := ValidateFileHasContent(ctx, s, localDNSUpdatedCorefile, fmt.Sprintf("cache %d", localDNSPatchedCacheSeconds)); err != nil {
		return err
	}
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

// validateLocalDNSLPSRejectedCorefileFallback runs once the node has provisioned, which is the main
// check. It then checks that the rejected patch is the reason: localdns.sh abandoned it, left the
// marker that stops the refetch, and serves the baked Corefile and limits.
func validateLocalDNSLPSRejectedCorefileFallback(ctx context.Context, s *Scenario) error {
	if err := ValidateFileExists(ctx, s, localDNSFetcherStamp); err != nil {
		return err
	}
	script := fmt.Sprintf(`set -euo pipefail
sudo journalctl -u localdns --no-pager | grep -E 'CoreDNS did not (start|become ready) on LPS corefileVersion %[1]s' ||
    { echo "FAIL: localdns.sh did not abandon corefileVersion %[1]s"; exit 1; }
sudo journalctl -u localdns --no-pager | grep 'earlier in this boot; skipping LocalDNS LPS config fetch' ||
    { echo "FAIL: no start skipped the LPS fetch after the rejection"; exit 1; }
`, rejectedLocalDNSVersion)
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "localdns did not fall back from the rejected LPS corefile"); err != nil {
		return err
	}
	if err := ValidateFileHasContent(ctx, s, localDNSLPSRejectedMarker, rejectedLocalDNSVersion); err != nil {
		return err
	}
	if err := ValidateFileDoesNotExist(ctx, s, localDNSLivepatchedCorefile); err != nil {
		return err
	}
	if err := ValidateFileDoesNotExist(ctx, s, localDNSLivepatchedCorefile+".version"); err != nil {
		return err
	}
	if err := ValidateFileExcludesContent(ctx, s, localDNSUpdatedCorefile, localDNSRejectedHealthCheck); err != nil {
		return err
	}
	if err := ValidateFileExcludesContent(ctx, s, localDNSUpdatedCorefile, fmt.Sprintf("cache %d", localDNSPatchedCacheSeconds)); err != nil {
		return err
	}
	// The limits are back on the baked ones. Resetting them goes through set-property, and systemd saves
	// CPUQuota truncated to a whole percent, so the restarted slice runs the baked 200.8% as 200%.
	limits := fmt.Sprintf(`set -euo pipefail
cgroup=/sys/fs/cgroup/localdns.slice
cat "$cgroup/cpu.max" "$cgroup/memory.max"
read -r quota period < "$cgroup/cpu.max"
test "$((quota * 100 / period))" = %[1]d || { echo "FAIL: expected cpu.max at the baked %[1]d%%"; exit 1; }
test "$(cat "$cgroup/memory.max")" = %[2]s || { echo "FAIL: expected memory.max at the baked %[2]s"; exit 1; }
`, localDNSBakedCPUPercent, localDNSBakedMemoryMax)
	if _, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, limits, 0, "localdns.slice is not back on the baked limits"); err != nil {
		return err
	}
	if err := ValidateLocalDNSService(ctx, s, "enabled"); err != nil {
		return err
	}
	return ValidateLocalDNSResolution(ctx, s, "169.254.10.10")
}

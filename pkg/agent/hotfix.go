// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/parts"
)

const hotfixConfigFile = "linux/cloud-init/hotfix.json"

// HotfixWriteFile represents a single write_files entry to inject into the
// EnableScriptlessCSECmd section of nodecustomdata.yml for hotfix purposes.
type HotfixWriteFile struct {
	Path        string
	Permissions string
	VarKey      string
}

// hotfixVarKeyToPath maps each cloudInitData variable property key to its
// on-disk destination path. This mirrors the write_files entries in the
// traditional section of nodecustomdata.yml.
var hotfixVarKeyToPath = map[string]string{
	// CSE helpers
	"provisionSource": cseHelpersScriptFilepath,
	// CSE helpers — distro variants (all write to same distro path)
	"provisionSourceUbuntu":     cseHelpersScriptDistroFilepath,
	"provisionSourceMariner":    cseHelpersScriptDistroFilepath,
	"provisionSourceAzlOSGuard": cseHelpersScriptDistroFilepath,
	"provisionSourceFlatcar":    cseHelpersScriptDistroFilepath,
	"provisionSourceACL":        cseHelpersScriptDistroFilepath,
	// CSE install
	"provisionInstalls": cseInstallScriptFilepath,
	// CSE install — distro variants
	"provisionInstallsUbuntu":     cseInstallScriptDistroFilepath,
	"provisionInstallsMariner":    cseInstallScriptDistroFilepath,
	"provisionInstallsAzlOSGuard": cseInstallScriptDistroFilepath,
	"provisionInstallsFlatcar":    cseInstallScriptDistroFilepath,
	"provisionInstallsACL":        cseInstallScriptDistroFilepath,
	// CSE config / main / start
	"provisionConfigs":    cseConfigScriptFilepath,
	"provisionScript":     "/opt/azure/containers/provision.sh",
	"provisionStartScript": "/opt/azure/containers/provision_start.sh",
	// Python scripts
	"provisionRedactCloudConfig": "/opt/azure/containers/provision_redact_cloud_config.py",
	"provisionSendLogs":          "/opt/azure/containers/provision_send_logs.py",
	// Other scripts
	"reconcilePrivateHostsScript":           "/opt/azure/containers/reconcilePrivateHosts.sh",
	"bindMountScript":                       "/opt/azure/containers/bind-mount.sh",
	"migPartitionScript":                    "/opt/azure/containers/mig-partition.sh",
	"dhcpv6ConfigurationScript":             dhcpV6ConfigCSEScriptFilepath,
	"ensureIMDSRestrictionScript":           "/opt/azure/containers/ensure_imds_restriction.sh",
	"ensureNoDupEbtablesScript":             "/opt/azure/containers/ensure-no-dup.sh",
	"cloudInitStatusCheckScript":            "/opt/azure/containers/cloud-init-status-check.sh",
	"measureTLSBootstrappingLatencyScript":  "/opt/azure/containers/measure-tls-bootstrapping-latency.sh",
	"validateKubeletCredentialsScript":      "/opt/azure/containers/validate-kubelet-credentials.sh",
	"customSearchDomainsScript":             customSearchDomainsCSEScriptFilepath,
	"configureAzureNetworkScript":           "/opt/azure-network/configure-azure-network.sh",
	"initAKSCustomCloud":                    initAKSCustomCloudFilepath,
	// Systemd services
	"kubeletSystemdService":                  "/etc/systemd/system/kubelet.service",
	"reconcilePrivateHostsService":           "/etc/systemd/system/reconcile-private-hosts.service",
	"bindMountSystemdService":                "/etc/systemd/system/bind-mount.service",
	"dhcpv6SystemdService":                   dhcpV6ServiceCSEScriptFilepath,
	"migPartitionSystemdService":             "/etc/systemd/system/mig-partition.service",
	"secureTLSBootstrapService":              "/etc/systemd/system/secure-tls-bootstrap.service",
	"ensureNoDupEbtablesService":             "/etc/systemd/system/ensure-no-dup.service",
	"measureTLSBootstrappingLatencyService":  "/etc/systemd/system/measure-tls-bootstrapping-latency.service",
	// Distro-specific scripts
	"snapshotUpdateScript":       "/opt/azure/containers/ubuntu-snapshot-update.sh",
	"snapshotUpdateService":      "/etc/systemd/system/snapshot-update.service",
	"snapshotUpdateTimer":        "/etc/systemd/system/snapshot-update.timer",
	"packageUpdateScriptMariner": "/opt/azure/containers/mariner-package-update.sh",
	"packageUpdateServiceMariner": "/etc/systemd/system/snapshot-update.service",
	"packageUpdateTimerMariner":   "/etc/systemd/system/snapshot-update.timer",
	// Component manifest
	"componentManifestFile": "/opt/azure/manifest.json",
	// Azure network udev rule
	"azureNetworkUdevRule": "/etc/udev/rules.d/99-azure-network.rules",
}

// loadHotfixConfig reads the hotfix.json file from the embedded filesystem
// and returns the list of varkeys that should be hotfixed.
func loadHotfixConfig() ([]string, error) {
	data, err := parts.Templates.ReadFile(hotfixConfigFile)
	if err != nil {
		return nil, fmt.Errorf("reading hotfix config: %w", err)
	}

	// Trim whitespace to handle trailing newlines
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "[]" || trimmed == "" {
		return nil, nil
	}

	var varKeys []string
	if err := json.Unmarshal([]byte(trimmed), &varKeys); err != nil {
		return nil, fmt.Errorf("parsing hotfix config: %w", err)
	}
	return varKeys, nil
}

// getHotfixWriteFiles returns the list of write_files entries to inject into
// the EnableScriptlessCSECmd section. Returns nil if no hotfix is active.
func getHotfixWriteFiles() []HotfixWriteFile {
	varKeys, err := loadHotfixConfig()
	if err != nil || len(varKeys) == 0 {
		return nil
	}

	var files []HotfixWriteFile
	for _, key := range varKeys {
		path, ok := hotfixVarKeyToPath[key]
		if !ok {
			continue
		}
		perm := "0744"
		if strings.HasSuffix(path, ".service") || strings.HasSuffix(path, ".timer") ||
			strings.HasSuffix(path, ".rules") || strings.HasSuffix(path, ".json") {
			perm = "0644"
		}
		files = append(files, HotfixWriteFile{
			Path:        path,
			Permissions: perm,
			VarKey:      key,
		})
	}
	return files
}

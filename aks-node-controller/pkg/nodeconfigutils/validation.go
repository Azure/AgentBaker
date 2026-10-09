package nodeconfigutils

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Masterminds/semver/v3"
)

// ValidateAndNormalizeConfiguration validates and updates provisioning settings in place.
// Producers call it before serialization; ANC calls it before rendering. It is idempotent
// and does not depend on the producer's or node's environment.
func ValidateAndNormalizeConfiguration(cfg *aksnodeconfigv1.Configuration) error {
	if cfg == nil {
		return fmt.Errorf("AKSNodeConfig is required")
	}
	if cfg.Version != "v0" && cfg.Version != "v1" {
		return fmt.Errorf("unsupported version: %s", cfg.Version)
	}
	if err := validateCustomLinuxOSConfig(cfg.GetCustomLinuxOsConfig()); err != nil {
		return err
	}

	kubelet := cfg.GetKubeletConfig()
	if kubelet == nil {
		return nil
	}
	version, err := semver.NewVersion(cfg.KubernetesVersion)
	if err != nil {
		return fmt.Errorf("invalid kubernetes_version: %w", err)
	}
	gates, err := parseFeatureGates(kubelet.KubeletFlags["--feature-gates"])
	if err != nil {
		return err
	}

	flags := kubelet.KubeletFlags
	if flags == nil {
		flags = make(map[string]string)
		kubelet.KubeletFlags = flags
	}
	for _, flag := range []string{
		"--dynamic-config-dir", "--non-masquerade-cidr",
		"--cni-bin-dir", "--cni-cache-dir", "--cni-conf-dir", "--docker-endpoint",
		"--image-pull-progress-deadline", "--network-plugin", "--network-plugin-mtu",
	} {
		delete(flags, flag)
	}
	normalizeFeatureGateFlag(kubelet, gates, version)
	kubeReserved, systemReserved := normalizeReservedCgroupFlags(kubelet)
	if version.GreaterThanEqual(semver.MustParse("1.34.0")) {
		delete(flags, "--streaming-connection-idle-timeout")
	}
	normalizeKubeletConfigFile(kubelet.GetKubeletConfigFileConfig(), version, kubeReserved, systemReserved)
	return nil
}

func validateCustomLinuxOSConfig(osConfig *aksnodeconfigv1.CustomLinuxOsConfig) error {
	for _, field := range []struct {
		name, value string
		allowed     []string
	}{
		{"transparent_hugepage_support", osConfig.GetTransparentHugepageSupport(), []string{"always", "madvise", "never"}},
		{"transparent_defrag", osConfig.GetTransparentDefrag(), []string{"always", "defer", "defer+madvise", "madvise", "never"}},
	} {
		if field.value != "" && !slices.Contains(field.allowed, field.value) {
			return fmt.Errorf("custom_linux_os_config.%s value %q is invalid; allowed values are: %s",
				field.name, field.value, strings.Join(field.allowed, ", "))
		}
	}
	return nil
}

func parseFeatureGates(raw string) (map[string]bool, error) {
	gates := make(map[string]bool)
	for _, entry := range strings.Split(raw, ",") {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		key, value, ok := strings.Cut(entry, "=")
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if !ok || strings.TrimSpace(key) == "" || err != nil {
			return nil, fmt.Errorf("invalid kubelet feature gate %q", entry)
		}
		gates[strings.TrimSpace(key)] = enabled
	}
	return gates, nil
}

func normalizeFeatureGateFlag(kubelet *aksnodeconfigv1.KubeletConfig, gates map[string]bool, version *semver.Version) {
	flags := kubelet.KubeletFlags
	servingRotation := kubelet.EnableKubeletConfigFile && kubelet.GetKubeletConfigFileConfig().GetServerTlsBootstrap()
	rotationFlag, hasRotationFlag := flags["--rotate-server-certificates"]
	if hasRotationFlag {
		servingRotation = rotationFlag == "true"
	}
	_, hasGateFlag := flags["--feature-gates"]
	// Do not introduce a CLI map that could override an existing config-file map.
	if hasGateFlag || hasRotationFlag || !kubelet.EnableKubeletConfigFile {
		normalizeFeatureGates(gates, version, servingRotation)
	}
	keys := make([]string, 0, len(gates))
	for key := range gates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%t", key, gates[key]))
	}
	if len(pairs) == 0 {
		delete(flags, "--feature-gates")
	} else {
		flags["--feature-gates"] = strings.Join(pairs, ",")
	}
}

func normalizeReservedCgroupFlags(kubelet *aksnodeconfigv1.KubeletConfig) (string, string) {
	flags := kubelet.KubeletFlags
	var enforced []string
	if kubelet.EnableKubeletConfigFile {
		enforced = kubelet.GetKubeletConfigFileConfig().GetEnforceNodeAllocatable()
	}
	// Command-line enforcement overrides the config file when both are provided.
	if raw, ok := flags["--enforce-node-allocatable"]; ok {
		raw = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(raw), "["), "]")
		enforced = strings.Split(raw, ",")
	}
	kubeReserved, systemReserved := reservedCgroups(enforced)
	delete(flags, "--kube-reserved-cgroup")
	delete(flags, "--system-reserved-cgroup")
	if _, ok := flags["--enforce-node-allocatable"]; ok || !kubelet.EnableKubeletConfigFile {
		if kubeReserved != "" {
			flags["--kube-reserved-cgroup"] = kubeReserved
			flags["--system-reserved-cgroup"] = systemReserved
		}
	}
	return kubeReserved, systemReserved
}

func normalizeKubeletConfigFile(fileConfig *aksnodeconfigv1.KubeletConfigFileConfig, version *semver.Version,
	kubeReserved, systemReserved string) {
	if fileConfig == nil {
		return
	}
	if fileConfig.FeatureGates == nil {
		fileConfig.FeatureGates = make(map[string]bool)
	}
	normalizeFeatureGates(fileConfig.FeatureGates, version, fileConfig.ServerTlsBootstrap)
	fileConfig.KubeReservedCgroup = kubeReserved
	fileConfig.SystemReservedCgroup = systemReserved
	if version.GreaterThanEqual(semver.MustParse("1.34.0")) {
		fileConfig.StreamingConnectionIdleTimeout = ""
	}
}

func normalizeFeatureGates(gates map[string]bool, version *semver.Version, servingRotation bool) {
	if servingRotation {
		gates["RotateKubeletServerCertificate"] = true
	}
	if version.GreaterThanEqual(semver.MustParse("1.24.0")) {
		delete(gates, "DynamicKubeletConfig")
	} else if version.GreaterThanEqual(semver.MustParse("1.11.0")) {
		gates["DynamicKubeletConfig"] = false
	}
	if version.GreaterThanEqual(semver.MustParse("1.20.0")) && version.LessThan(semver.MustParse("1.25.0")) {
		gates["DisableAcceleratorUsageMetrics"] = false
	}
}

func reservedCgroups(enforced []string) (string, string) {
	var kube, system bool
	for _, value := range enforced {
		switch strings.TrimSpace(value) {
		case "kube-reserved":
			kube = true
		case "system-reserved":
			system = true
		}
	}
	if kube && system {
		// Keep aligned with cse_helpers.sh::ensureKubeletCgroupHierarchy.
		return "/kubereserved.slice", "/system.slice"
	}
	return "", ""
}

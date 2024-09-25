/*
Portions Copyright (c) Microsoft Corporation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// All the helper functions should be hosted by another public repo later. (e.g. agentbaker)
// Helper functions in this file will be called by bootstrappers to populate nb contract payload.
package nbcontractv1

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/blang/semver"
)

const numInPair = 2

// GetLoadBalancerSKI returns the LoadBalancerSku enum based on the input string.
func GetLoadBalancerSKU(sku string) LoadBalancerConfig_LoadBalancerSku {
	if strings.EqualFold(sku, "Standard") {
		return LoadBalancerConfig_STANDARD
	} else if strings.EqualFold(sku, "Basic") {
		return LoadBalancerConfig_BASIC
	}

	return LoadBalancerConfig_UNSPECIFIED
}

// GetNetworkPluginType returns the NetworkPluginType enum based on the input string.
func GetNetworkPluginType(networkPlugin string) NetworkPlugin {
	if strings.EqualFold(networkPlugin, "azure") {
		return NetworkPlugin_NP_AZURE
	} else if strings.EqualFold(networkPlugin, "kubenet") {
		return NetworkPlugin_NP_KUBENET
	}

	return NetworkPlugin_NP_NONE
}

// GetNetworkPolicyType returns the NetworkPolicyType enum based on the input string.
func GetNetworkPolicyType(networkPolicy string) NetworkPolicy {
	if strings.EqualFold(networkPolicy, "azure") {
		return NetworkPolicy_NPO_AZURE
	} else if strings.EqualFold(networkPolicy, "calico") {
		return NetworkPolicy_NPO_CALICO
	}

	return NetworkPolicy_NPO_NONE
}

// GetDefaultOutboundCommand returns a default outbound traffic command.
func GetDefaultOutboundCommand() string {
	return "curl -v --insecure --proxy-insecure https://mcr.microsoft.com/v2/"
}

func GetKubeletNodeLabels(agentPool *datamodel.AgentPoolProfile) map[string]string {
	kubeletLabels := map[string]string{
		"agentpool":                      agentPool.Name,
		"kubernetes.azure.com/agentpool": agentPool.Name,
	}
	for key, val := range agentPool.CustomNodeLabels {
		kubeletLabels[key] = val
	}
	return kubeletLabels
}

// NOTE: The following functions are slightly modified versions of those that are already in agent/utils.go.
// Other functions from agent/utils.go will also need to be added/merged here.

// GetOutBoundCmd returns a proper outbound traffic command based on some cloud and Linux distro configs.
func GetOutBoundCmd(nbc *datamodel.NodeBootstrappingConfiguration) string {
	cs := nbc.ContainerService
	if cs.Properties.FeatureFlags.IsFeatureEnabled("BlockOutboundInternet") {
		return ""
	}

	if strings.EqualFold(nbc.OutboundType, datamodel.OutboundTypeBlock) || strings.EqualFold(nbc.OutboundType, datamodel.OutboundTypeNone) {
		return ""
	}

	var registry string
	switch {
	case nbc.CloudSpecConfig.CloudName == datamodel.AzureChinaCloud:
		registry = `gcr.azk8s.cn`
	case cs.IsAKSCustomCloud():
		registry = cs.Properties.CustomCloudEnv.McrURL
	default:
		registry = `mcr.microsoft.com`
	}

	if registry == "" {
		return ""
	}

	connectivityCheckCommand := `curl -v --insecure --proxy-insecure https://` + registry + `/v2/`

	return connectivityCheckCommand
}

// GetOrderedKubeletConfigFlagString returns an ordered string of key/val pairs.
// copied from AKS-Engine and filter out flags that already translated to config file.
func GetKubeletConfigFlag(k map[string]string, cs *datamodel.ContainerService, profile *datamodel.AgentPoolProfile,
	kubeletConfigFileToggleEnabled bool) map[string]string {
	/* NOTE(mainred): kubeConfigFile now relies on CustomKubeletConfig, while custom configuration is not
	compatible with CustomKubeletConfig. When custom configuration is set we want to override every
	configuration with the customized one. */
	kubeletCustomConfigurations := getKubeletCustomConfiguration(cs.Properties)
	if kubeletCustomConfigurations != nil {
		return getKubeletConfigFlagWithCustomConfiguration(kubeletCustomConfigurations, k)
	}

	if k == nil {
		return nil
	}
	// Always force remove of dynamic-config-dir.
	kubeletConfigFileEnabled := agent.IsKubeletConfigFileEnabled(cs, profile, kubeletConfigFileToggleEnabled)
	kubeletConfigFlags := map[string]string{}
	ommitedKubletConfigFlags := datamodel.GetCommandLineOmittedKubeletConfigFlags()
	for key := range k {
		if !kubeletConfigFileEnabled || !agent.TranslatedKubeletConfigFlags[key] {
			if !ommitedKubletConfigFlags[key] {
				kubeletConfigFlags[key] = k[key]
			}
		}
	}
	return kubeletConfigFlags
}

func getKubeletConfigFlagWithCustomConfiguration(customConfig, defaultConfig map[string]string) map[string]string {
	config := customConfig

	for k, v := range defaultConfig {
		// add key-value only when the flag does not exist in custom config.
		if _, ok := config[k]; !ok {
			config[k] = v
		}
	}

	ommitedKubletConfigFlags := datamodel.GetCommandLineOmittedKubeletConfigFlags()
	for key := range config {
		if ommitedKubletConfigFlags[key] {
			delete(config, key)
		}
	}
	return config
}

func getKubeletCustomConfiguration(properties *datamodel.Properties) map[string]string {
	if properties.CustomConfiguration == nil || properties.CustomConfiguration.KubernetesConfigurations == nil {
		return nil
	}
	kubeletConfigurations, ok := properties.CustomConfiguration.KubernetesConfigurations["kubelet"]
	if !ok {
		return nil
	}
	if kubeletConfigurations.Config == nil {
		return nil
	}
	// empty config is treated as nil.
	if len(kubeletConfigurations.Config) == 0 {
		return nil
	}
	return kubeletConfigurations.Config
}

func ValidateAndSetLinuxKubeletFlags(kubeletFlags map[string]string, cs *datamodel.ContainerService, profile *datamodel.AgentPoolProfile) {
	// If using kubelet config file, disable DynamicKubeletConfig feature gate and remove dynamic-config-dir
	// we should only allow users to configure from API (20201101 and later)
	dockerShimFlags := []string{
		"--cni-bin-dir",
		"--cni-cache-dir",
		"--cni-conf-dir",
		"--docker-endpoint",
		"--image-pull-progress-deadline",
		"--network-plugin",
		"--network-plugin-mtu",
	}

	if kubeletFlags != nil {
		delete(kubeletFlags, "--dynamic-config-dir")
		delete(kubeletFlags, "--non-masquerade-cidr")
		if profile != nil && profile.KubernetesConfig != nil &&
			profile.KubernetesConfig.ContainerRuntime != "" &&
			profile.KubernetesConfig.ContainerRuntime == "containerd" {
			for _, flag := range dockerShimFlags {
				delete(kubeletFlags, flag)
			}
		}
		if IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, "1.24.0") {
			kubeletFlags["--feature-gates"] = removeFeatureGateString(kubeletFlags["--feature-gates"], "DynamicKubeletConfig")
		} else if IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, "1.11.0") {
			kubeletFlags["--feature-gates"] = addFeatureGateString(kubeletFlags["--feature-gates"], "DynamicKubeletConfig", false)
		}

		/* ContainerInsights depends on GPU accelerator Usage metrics from Kubelet cAdvisor endpoint but
		deprecation of this feature moved to beta which breaks the ContainerInsights customers with K8s
		 version 1.20 or higher */
		/* Until Container Insights move to new API adding this feature gate to get the GPU metrics
		continue to work */
		/* Reference -
		https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/1867-disable-accelerator-usage-metrics */
		if IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, "1.20.0") &&
			!IsKubernetesVersionGe(cs.Properties.OrchestratorProfile.OrchestratorVersion, "1.25.0") {
			kubeletFlags["--feature-gates"] = addFeatureGateString(kubeletFlags["--feature-gates"], "DisableAcceleratorUsageMetrics", false)
		}
	}
}

// IsKubernetesVersionGe returns true if actualVersion is greater than or equal to version.
func IsKubernetesVersionGe(actualVersion, version string) bool {
	v1, _ := semver.Make(actualVersion)
	v2, _ := semver.Make(version)
	return v1.GE(v2)
}

func strKeyValToMapBool(str string, strDelim string, pairDelim string) map[string]bool {
	m := make(map[string]bool)
	pairs := strings.Split(str, strDelim)
	for _, pairRaw := range pairs {
		pair := strings.Split(pairRaw, pairDelim)
		if len(pair) == numInPair {
			key := strings.TrimSpace(pair[0])
			val := strings.TrimSpace(pair[1])
			m[key] = strToBool(val)
		}
	}
	return m
}

func removeFeatureGateString(featureGates string, key string) string {
	fgMap := strKeyValToMapBool(featureGates, ",", "=")
	delete(fgMap, key)
	keys := make([]string, 0, len(fgMap))
	for k := range fgMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%t", k, fgMap[k]))
	}
	return strings.Join(pairs, ",")
}

func addFeatureGateString(featureGates string, key string, value bool) string {
	fgMap := strKeyValToMapBool(featureGates, ",", "=")
	fgMap[key] = value
	keys := make([]string, 0, len(fgMap))
	for k := range fgMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%t", k, fgMap[k]))
	}
	return strings.Join(pairs, ",")
}

func strToBool(str string) bool {
	b, _ := strconv.ParseBool(str)
	return b
}

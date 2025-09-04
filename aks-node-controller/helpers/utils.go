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
package helpers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/blang/semver"
)

const numInPair = 2

// GetLoadBalancerSKI returns the LoadBalancerSku enum based on the input string.
func GetLoadBalancerSKU(sku string) aksnodeconfigv1.LoadBalancerSku {
	if strings.EqualFold(sku, "Standard") {
		return aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_STANDARD
	} else if strings.EqualFold(sku, "Basic") {
		return aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_BASIC
	}

	return aksnodeconfigv1.LoadBalancerSku_LOAD_BALANCER_SKU_UNSPECIFIED
}

// GetNetworkPluginType returns the NetworkPluginType enum based on the input string.
func GetNetworkPluginType(networkPlugin string) aksnodeconfigv1.NetworkPlugin {
	if strings.EqualFold(networkPlugin, "azure") {
		return aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_AZURE
	} else if strings.EqualFold(networkPlugin, "kubenet") {
		return aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_KUBENET
	}

	return aksnodeconfigv1.NetworkPlugin_NETWORK_PLUGIN_NONE
}

// GetNetworkPolicyType returns the NetworkPolicyType enum based on the input string.
func GetNetworkPolicyType(networkPolicy string) aksnodeconfigv1.NetworkPolicy {
	if strings.EqualFold(networkPolicy, "azure") {
		return aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_AZURE
	} else if strings.EqualFold(networkPolicy, "calico") {
		return aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_CALICO
	}

	return aksnodeconfigv1.NetworkPolicy_NETWORK_POLICY_NONE
}

// GetDefaultOutboundCommand returns a default outbound traffic command.
func GetDefaultOutboundCommand() string {
	return "curl -v --insecure --proxy-insecure https://mcr.microsoft.com/v2/"
}

func isKubeletServingCertificateRotationEnabled(kubeletFlags map[string]string) bool {
	if kubeletFlags == nil {
		return false
	}
	return kubeletFlags["--rotate-server-certificates"] == "true"
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

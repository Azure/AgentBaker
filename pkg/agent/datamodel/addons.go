// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"fmt"
	"strings"

	"github.com/Azure/aks-engine/pkg/api"
)

// GetClusterAutoscalerNodesConfig returns the cluster-autoscaler runtime configuration flag for a nodepool
func GetClusterAutoscalerNodesConfig(addon api.KubernetesAddon, cs *ContainerService) string {
	var ret string
	for _, pool := range addon.Pools {
		nodepoolName := cs.Properties.GetAgentVMPrefix(cs.Properties.GetAgentPoolByName(pool.Name), cs.Properties.GetAgentPoolIndexByName(pool.Name))
		ret += fmt.Sprintf("        - --nodes=%s:%s:%s\n", pool.Config["min-nodes"], pool.Config["max-nodes"], nodepoolName)
	}
	if ret != "" {
		ret = strings.TrimRight(ret, "\n")
	}
	return ret
}

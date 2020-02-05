// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"github.com/Azure/aks-engine/pkg/api"
)

func getAgentCustomDataStr(cs *api.ContainerService, profile *api.AgentPoolProfile) string {
	t, err := InitializeTemplateGenerator(Context{}, cs)

	if err != nil {
		panic(err)
	}
	if profile.IsWindows() {
		return getCustomDataFromJSON(t.GetKubernetesWindowsNodeCustomDataJSONObject(cs, profile))
	}
	return getCustomDataFromJSON(t.GetKubernetesLinuxNodeCustomDataJSONObject(cs, profile))
}

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func TestAzureLinuxV3KataCCRequiresRuntimeIntegration(t *testing.T) {
	for _, distro := range []datamodel.Distro{datamodel.AKSAzureLinuxV3Gen2KataCC, datamodel.AKSAzureLinuxV3Gen2Kata, datamodel.AKSCBLMarinerV2Gen2Kata} {
		t.Run(string(distro), func(t *testing.T) {
			config := &datamodel.NodeBootstrappingConfiguration{AgentPoolProfile: &datamodel.AgentPoolProfile{Distro: distro}}
			err := ValidateAndSetLinuxNodeBootstrappingConfigurationWithError(config)
			if distro == datamodel.AKSAzureLinuxV3Gen2KataCC {
				if err == nil || !strings.Contains(err.Error(), "OpenVMM runtime configuration") {
					t.Fatalf("must reject legacy configuration for the new image, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("existing image was blocked: %v", err)
			}
		})
	}
}

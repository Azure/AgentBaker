package agent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// Windows CSE polls this URI to confirm kubelet initialized after the node reset task started it,
// so the rendered value has to stay in sync with the kubelet flags rendered next to it.
func TestWindowsCustomDataRendersKubeletHealthzUri(t *testing.T) {
	cases := []struct {
		name          string
		kubeletConfig map[string]string
		expected      string
	}{
		{
			name:          "kubelet defaults when healthz is not configured",
			kubeletConfig: map[string]string{"--address": "0.0.0.0"},
			expected:      "http://127.0.0.1:10248/healthz",
		},
		{
			name: "overridden port and bind address",
			kubeletConfig: map[string]string{
				"--healthz-port":         "10267",
				"--healthz-bind-address": "127.0.0.2",
			},
			expected: "http://127.0.0.2:10267/healthz",
		},
		{
			name:          "empty when the healthz server is disabled",
			kubeletConfig: map[string]string{"--healthz-port": "0"},
			expected:      "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			cs := &datamodel.ContainerService{
				Location:   "eastus",
				Properties: datamodel.GetK8sDefaultProperties(true),
			}
			cs.Properties.OrchestratorProfile.OrchestratorVersion = "1.31.0"
			config := &datamodel.NodeBootstrappingConfiguration{
				ContainerService: cs,
				AgentPoolProfile: cs.Properties.AgentPoolProfiles[0],
				K8sComponents:    &datamodel.K8sComponents{},
				CloudSpecConfig:  datamodel.AzurePublicCloudSpecForTest,
				KubeletConfig:    c.kubeletConfig,
			}

			customData := InitializeTemplateGenerator().getWindowsNodeCustomDataJSONObject(config)

			expected := fmt.Sprintf(`$global:KubeletHealthzUri=\"%s\"`, c.expected)
			if !strings.Contains(customData, expected) {
				t.Fatalf("rendered Windows CustomData does not contain %q", expected)
			}
		})
	}
}

package agent

import (
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestRenderLinuxNodeCustomDataTemplateUsesBakerPlatformFunctions(t *testing.T) {
	template := []byte(`#cloud-config
write_files:
{{if IsACL}}
- path: /acl
{{else if IsAzlOSGuard}}
- path: /azlosguard
{{else if IsMariner}}
- path: /mariner
{{else if IsFlatcar}}
- path: /flatcar
{{else}}
- path: /ubuntu
{{end}}
`)
	tests := []struct {
		name     string
		distro   datamodel.Distro
		expected string
	}{
		{name: "Ubuntu", distro: datamodel.AKSUbuntuContainerd2204Gen2, expected: "/ubuntu"},
		{name: "Mariner", distro: datamodel.AKSAzureLinuxV3Gen2, expected: "/mariner"},
		{name: "ACL", distro: datamodel.AKSACLGen2TL, expected: "/acl"},
		{name: "OS Guard", distro: datamodel.AKSAzureLinuxV3OSGuardGen2FIPSTL, expected: "/azlosguard"},
		{name: "Flatcar", distro: datamodel.AKSFlatcarGen2, expected: "/flatcar"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rendered, err := RenderLinuxNodeCustomDataTemplate(
				template,
				newNodeCustomDataRenderConfig(test.distro),
			)

			require.NoError(t, err)
			require.Contains(t, rendered, "- path: "+test.expected)
			require.False(t, strings.Contains(rendered, "{{"))
		})
	}
}

func TestRenderLinuxNodeCustomDataTemplateRejectsMissingDependencies(t *testing.T) {
	tests := []struct {
		name   string
		remove func(*datamodel.NodeBootstrappingConfiguration)
	}{
		{
			name: "orchestrator profile",
			remove: func(config *datamodel.NodeBootstrappingConfiguration) {
				config.ContainerService.Properties.OrchestratorProfile = nil
			},
		},
		{
			name: "Kubernetes components",
			remove: func(config *datamodel.NodeBootstrappingConfiguration) {
				config.K8sComponents = nil
			},
		},
		{
			name: "cloud spec config",
			remove: func(config *datamodel.NodeBootstrappingConfiguration) {
				config.CloudSpecConfig = nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
			test.remove(config)

			_, err := RenderLinuxNodeCustomDataTemplate([]byte("#cloud-config\nwrite_files: []\n"), config)

			require.EqualError(t, err, "node bootstrapping configuration is incomplete")
		})
	}
}

func newNodeCustomDataRenderConfig(distro datamodel.Distro) *datamodel.NodeBootstrappingConfiguration {
	profile := &datamodel.AgentPoolProfile{
		Name:   "hotfix-render-test",
		OSType: datamodel.Linux,
		Distro: distro,
	}
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Location: "eastus",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorVersion: "1.29.0",
					OrchestratorType:    datamodel.Kubernetes,
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntimeConfig: map[string]string{},
					},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					FQDN: "hotfix-render.invalid",
				},
				AgentPoolProfiles: []*datamodel.AgentPoolProfile{profile},
			},
		},
		AgentPoolProfile: profile,
		CloudSpecConfig:  datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:    &datamodel.K8sComponents{},
		KubeletConfig:    map[string]string{},
	}
}

package agent

import (
	"encoding/json"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig(t *testing.T) {
	type VersionSupport struct {
		MinImageVersion string `json:"MinImageVersion"`
	}

	type ConfigVersionSupport struct {
		V1 VersionSupport `json:"v1"`
	}

	type Distro struct {
		Image                datamodel.SigImageConfig `json:"Image"`
		ConfigVersionSupport *ConfigVersionSupport    `json:"ConfigVersionSupport,omitempty"`
	}
	type DistrosConfig struct {
		Variables map[string]any              `json:"Variables"`
		Distros   map[datamodel.Distro]Distro `json:"Distros"`
	}
	agentBaker, err := NewAgentBaker()
	require.NoError(t, err)
	galleries := map[string]datamodel.SIGGalleryConfig{
		"AKSUbuntu": {
			GalleryName:   "aksubuntu",
			ResourceGroup: "resourcegroup",
		},
		"AKSCBLMariner": {
			GalleryName:   "akscblmariner",
			ResourceGroup: "resourcegroup",
		},
		"AKSAzureLinux": {
			GalleryName:   "aksazurelinux",
			ResourceGroup: "resourcegroup",
		},
		"AKSWindows": {
			GalleryName:   "akswindows",
			ResourceGroup: "resourcegroup",
		},
		"AKSUbuntuEdgeZone": {
			GalleryName:   "AKSUbuntuEdgeZone",
			ResourceGroup: "AKS-Ubuntu-EdgeZone",
		},
	}
	sigConfig := &datamodel.SIGConfig{
		TenantID:       "sometenantid",
		SubscriptionID: "somesubid",
		Galleries:      galleries,
	}
	config := &datamodel.NodeBootstrappingConfiguration{
		CloudSpecConfig:   datamodel.AzurePublicCloudSpecForTest,
		TenantID:          "tenantID",
		SubscriptionID:    "subID",
		ResourceGroupName: "resourceGroupName",
		SIGConfig:         *sigConfig,
	}
	configs, err := agentBaker.GetDistroSigImageConfig(config.SIGConfig, &datamodel.EnvironmentInfo{
		SubscriptionID: config.SubscriptionID,
		TenantID:       config.TenantID,
		Region:         "southcentralus",
	})
	result := DistrosConfig{
		Variables: map[string]any{
			"LatestLinuxVersion": datamodel.LinuxSIGImageVersion,
		},
		Distros: make(map[datamodel.Distro]Distro),
	}
	for k, v := range configs {
		distro := Distro{
			Image: v,
		}
		if v.Version != datamodel.LinuxSIGImageVersion {
			continue
		}
		if v.Version == datamodel.LinuxSIGImageVersion {
			distro.ConfigVersionSupport = &ConfigVersionSupport{
				V1: VersionSupport{
					MinImageVersion: v.Version,
				},
			}
		}
		result.Distros[k] = distro
	}

	configsData, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)
	t.Log(string(configsData))
}

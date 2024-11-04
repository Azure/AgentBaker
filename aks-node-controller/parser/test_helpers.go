package parser

import (
	"context"
	"io/fs"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getFile(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration, path string, expectedMode fs.FileMode) string {
	t.Helper()
	files, err := customData(context.TODO(), nbc)
	require.NoError(t, err)

	require.Contains(t, files, path)
	actual := files[path]
	assert.Equal(t, expectedMode, actual.Mode)

	return actual.Content
}

func Ptr[T any](input T) *T {
	return &input
}

func validNBC() *datamodel.NodeBootstrappingConfiguration {
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
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				CertificateProfile: &datamodel.CertificateProfile{
					CaCertificate: "test-ca-cert",
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{
					FQDN: "aks-timmy-wrightt-resource-82acd5-zr3wop33.hcp.australiaeast.azmk8s.io",
				},
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: "1.31.0",
					KubernetesConfig: &datamodel.KubernetesConfig{
						DockerBridgeSubnet: "1.1.1.1",
					},
				},
			},
		},
		CustomSecureTLSBootstrapAADServerAppID: "test-app-id",
		AgentPoolProfile: &datamodel.AgentPoolProfile{
			KubeletDiskType:         datamodel.TempDisk,
			AgentPoolWindowsProfile: &datamodel.AgentPoolWindowsProfile{},
			Distro:                  datamodel.AKSWindows2022ContainerdGen2,
		},
		SIGConfig: *sigConfig,
	}
}

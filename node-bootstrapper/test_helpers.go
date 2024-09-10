package main

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/fs"
	"testing"
)

func getFile(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration, path string, expectedMode fs.FileMode) string {
	t.Helper()
	files, err := customData(nbc)
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
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				CertificateProfile: &datamodel.CertificateProfile{
					CaCertificate: "test-ca-cert",
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
			KubeletDiskType: datamodel.TempDisk,
		},
	}
}

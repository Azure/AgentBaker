package main

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func Ptr[T any](input T) *T {
	return &input
}

func validAKSNodeConfig() *datamodel.NodeBootstrappingConfiguration {
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

package parser

import nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"

func getBaseTemplate() *nbcontractv1.Configuration {
	return &nbcontractv1.Configuration{
		ProvisionOutput:     "/var/log/azure/cluster-provision-cse-output.log",
		LinuxAdminUsername:  "azureuser",
		RepoDepotEndpoint:   "",
		MobyVersion:         "",
		TenantId:            "",
		KubernetesVersion:   "1.26.0",
		HyperkubeUrl:        "mcr.microsoft.com/oss/kubernetes/",
		KubeBinaryUrl:       "",
		CustomKubeBinaryUrl: "https://acs-mirror.azureedge.net/kubernetes/v1.26.0/binaries/kubernetes-node-linux-amd64.tar.gz",
		KubeproxyUrl:        "",
		CustomCloudConfig: &nbcontractv1.CustomCloudConfig{
			IsCustomCloud: false,
		},
		SshStatus: nbcontractv1.FeatureState_FEATURE_STATE_ENABLED,
	}
}

package parser

import (
	"text/template"

	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

func getBaseTemplate() *nbcontractv1.Configuration {
	return &nbcontractv1.Configuration{
		ProvisionOutput:    "/var/log/azure/cluster-provision-cse-output.log",
		LinuxAdminUsername: "azureuser",
		MobyVersion:        "",
		TenantId:           "",
		KubernetesVersion:  "1.26.0",
		HyperkubeUrl:       "mcr.microsoft.com/oss/kubernetes/",
		KubeBinaryConfig: &nbcontractv1.KubeBinaryConfig{
			KubeBinaryUrl:        "",
			CustomKubeBinaryUrl:  "https://acs-mirror.azureedge.net/kubernetes/v1.26.0/binaries/kubernetes-node-linux-amd64.tar.gz",
			PrivateKubeBinaryUrl: "",
		},
		KubeproxyUrl: "",
		SshStatus:    nbcontractv1.FeatureState_FEATURE_STATE_ENABLED,
	}
}

func getBoolStr(state nbcontractv1.FeatureState) string {
	if state == nbcontractv1.FeatureState_FEATURE_STATE_ENABLED {
		return "true"
	}

	return "false"
}

func getInverseBoolStr(state nbcontractv1.FeatureState) string {
	if state == nbcontractv1.FeatureState_FEATURE_STATE_ENABLED {
		return "false"
	}

	return "true"
}

func getFuncMap() template.FuncMap {
	return template.FuncMap{
		"getBoolStr":        getBoolStr,
		"getInverseBoolStr": getInverseBoolStr,
	}
}

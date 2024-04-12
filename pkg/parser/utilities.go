package parser

import (
	"text/template"

	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

func getBaseTemplate() *nbcontractv1.Configuration {
	return &nbcontractv1.Configuration{
		ProvisionOutput:    "/var/log/azure/cluster-provision-cse-output.log",
		LinuxAdminUsername: "azureuser",
		KubernetesVersion:  "1.26.0",
		KubeBinaryConfig: &nbcontractv1.KubeBinaryConfig{
			KubeBinaryUrl:        "",
			CustomKubeBinaryUrl:  "https://acs-mirror.azureedge.net/kubernetes/v1.26.0/binaries/kubernetes-node-linux-amd64.tar.gz",
			PrivateKubeBinaryUrl: "",
		},
		KubeProxyUrl: "",
		EnableSsh:    true,
	}
}

func getFuncMap() template.FuncMap {
	return template.FuncMap{}
}

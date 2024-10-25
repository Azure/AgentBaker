package e2e

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
)

func getBaseNodeBootstrappingConfiguration(clusterParams ClusterParams) (*datamodel.NodeBootstrappingConfiguration, error) {
	nbc := baseTemplate(config.Config.Location)
	nbc.ContainerService.Properties.CertificateProfile.CaCertificate = string(clusterParams.CACert)
	nbc.KubeletClientTLSBootstrapToken = &clusterParams.BootstrapToken
	nbc.ContainerService.Properties.HostedMasterProfile.FQDN = clusterParams.FQDN
	return nbc, nil
}

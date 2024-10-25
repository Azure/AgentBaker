package e2e

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
)

func getBaseNodeBootstrappingConfiguration(clusterParams ClusterParams) (*datamodel.NodeBootstrappingConfiguration, error) {
	nbc := baseTemplate(config.Config.Location)
	nbc.ContainerService.Properties.CertificateProfile.CaCertificate = string(clusterParams.CACert)

	bootstrapKubeconfig := string(clusterParams.BootstrapKubeconfig)

	bootstrapToken, err := extractKeyValuePair("token", bootstrapKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to extract bootstrap token via regex: %w", err)
	}

	bootstrapToken, err = strconv.Unquote(bootstrapToken)
	if err != nil {
		return nil, fmt.Errorf("failed to unquote bootstrap token: %w", err)
	}

	server, err := extractKeyValuePair("server", bootstrapKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to extract fqdn via regex: %w", err)
	}
	tokens := strings.Split(server, ":")
	if len(tokens) != 3 {
		return nil, fmt.Errorf("expected 3 tokens from fqdn %q, got %d", server, len(tokens))
	}
	// strip off the // prefix from https://
	fqdn := tokens[1][2:]

	nbc.KubeletClientTLSBootstrapToken = &bootstrapToken
	nbc.ContainerService.Properties.HostedMasterProfile.FQDN = fqdn

	return nbc, nil
}

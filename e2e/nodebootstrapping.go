package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbakere2e/config"
)

func getNodeBootstrapping(ctx context.Context, nbc *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error) {
	switch e2eMode {
	case "coverage":
		return getNodeBootstrappingForCoverage(nbc)
	default:
		return getNodeBootstrappingForValidation(ctx, nbc)
	}
}

func getNodeBootstrappingForCoverage(nbc *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error) {
	payload, err := json.Marshal(nbc)
	if err != nil {
		log.Fatalf("failed to marshal nbc, error: %s", err)
	}
	res, err := http.Post("http://localhost:8080/getnodebootstrapdata", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalf("failed to retrieve node bootstrapping data, error: %s", err)
	}

	if res.StatusCode != http.StatusOK {
		log.Fatalf("failed to retrieve node bootstrapping data, status code: %d", res.StatusCode)
	}

	var nodeBootstrapping *datamodel.NodeBootstrapping
	err = json.NewDecoder(res.Body).Decode(&nodeBootstrapping)
	if err != nil {
		log.Fatalf("failed to unmarshal node bootstrapping data, error: %s", err)
	}

	return nodeBootstrapping, nil
}

func getNodeBootstrappingForValidation(ctx context.Context, nbc *datamodel.NodeBootstrappingConfiguration) (*datamodel.NodeBootstrapping, error) {
	ab, err := agent.NewAgentBaker()
	if err != nil {
		return nil, err
	}
	nodeBootstrapping, err := ab.GetNodeBootstrapping(ctx, nbc)
	if err != nil {
		return nil, err
	}
	return nodeBootstrapping, nil
}

func getBaseNodeBootstrappingConfiguration(clusterParams map[string]string) (*datamodel.NodeBootstrappingConfiguration, error) {
	nbc := baseTemplate(config.Config.Location)
	nbc.ContainerService.Properties.CertificateProfile.CaCertificate = clusterParams["/etc/kubernetes/certs/ca.crt"]

	bootstrapKubeconfig := clusterParams["/var/lib/kubelet/bootstrap-kubeconfig"]

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

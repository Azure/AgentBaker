package agent

import (
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/stretchr/testify/assert"
)

func MutateMap[T any](ts []T, f func(T) T) []T {
	for i := range ts {
		ts[i] = f(ts[i])
	}
	return ts
}

func trim(t string) string { return strings.Trim(t, " ") }

func TestNoProxyIsSet(t *testing.T) {
	config := &datamodel.NodeBootstrappingConfiguration{
		HTTPProxyConfig: &datamodel.HTTPProxyConfig{
			NoProxy: &[]string{"no_proxy_domain", "another_domain"},
		},
		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				HostedMasterProfile: &datamodel.HostedMasterProfile{},
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntimeConfig: map[string]string{},
					},
				},
			},
		},
		AgentPoolProfile: &datamodel.AgentPoolProfile{},
	}

	proxyVars := getProxyVariables(config)
	vars := strings.Split(proxyVars, ";")
	MutateMap(vars, trim)

	// curl looks for both of these, and prefers the lower case one. We set both because they are both set in /etc/environment and
	// we want to make sure both are overwritten
	assert.Contains(t, vars, "export NO_PROXY=\"no_proxy_domain,another_domain\"")
	assert.Contains(t, vars, "export no_proxy=\"no_proxy_domain,another_domain\"")
}

func TestHttpProxyIsSet(t *testing.T) {
	config := &datamodel.NodeBootstrappingConfiguration{
		HTTPProxyConfig: &datamodel.HTTPProxyConfig{
			HTTPProxy: to.StringPtr("https://httpproxy:80/"),
		},
		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				HostedMasterProfile: &datamodel.HostedMasterProfile{},
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntimeConfig: map[string]string{},
					},
				},
			},
		},
		AgentPoolProfile: &datamodel.AgentPoolProfile{},
	}

	proxyVars := getProxyVariables(config)
	vars := strings.Split(proxyVars, ";")
	MutateMap(vars, trim)

	// curl looks for both of these, and prefers the lower case one. We set both because they are both set in /etc/environment and
	// we want to make sure both are overwritten
	assert.Contains(t, vars, "export HTTP_PROXY=\"https://httpproxy:80/\"")
	assert.Contains(t, vars, "export http_proxy=\"https://httpproxy:80/\"")
}

func TestHttpsProxyIsSet(t *testing.T) {
	config := &datamodel.NodeBootstrappingConfiguration{
		HTTPProxyConfig: &datamodel.HTTPProxyConfig{
			HTTPSProxy: to.StringPtr("https://httpproxy:80/"),
		},
		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				HostedMasterProfile: &datamodel.HostedMasterProfile{},
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					KubernetesConfig: &datamodel.KubernetesConfig{
						ContainerRuntimeConfig: map[string]string{},
					},
				},
			},
		},
		AgentPoolProfile: &datamodel.AgentPoolProfile{},
	}

	proxyVars := getProxyVariables(config)
	vars := strings.Split(proxyVars, ";")
	MutateMap(vars, trim)

	// curl looks for both of these, and prefers the lower case one. We set both because they are both set in /etc/environment and
	// we want to make sure both are overwritten
	assert.Contains(t, vars, "export HTTPS_PROXY=\"https://httpproxy:80/\"")
	assert.Contains(t, vars, "export https_proxy=\"https://httpproxy:80/\"")
}

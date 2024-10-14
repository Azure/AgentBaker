package main

import (
	"fmt"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

func useKubeconfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	switch config.AgentPoolProfile.BootstrappingMethod {
	case datamodel.UseArcMsiToMakeCSR:
		if err := useBootstrappingKubeConfig(config, files); err != nil {
			return err
		}
		if err := useArcTokenSh(config, files); err != nil {
			return err
		}

	case datamodel.UseArcMsiDirectly:
		if err := useHardCodedKubeconfig(config, files); err != nil {
			return err
		}
		if err := useArcTokenSh(config, files); err != nil {
			return err
		}

	case datamodel.UseAzureMsiDirectly:
		if err := useHardCodedKubeconfig(config, files); err != nil {
			return err
		}
		if err := useAzureTokenSh(config, files); err != nil {
			return err
		}

	case datamodel.UseAzureMsiToMakeCSR:
		if err := useBootstrappingKubeConfig(config, files); err != nil {
			return err
		}
		if err := useAzureTokenSh(config, files); err != nil {
			return err
		}

	case datamodel.UseTlsBootstrapToken, datamodel.UseSecureTLSBootstrapping:
		if err := useBootstrappingKubeConfig(config, files); err != nil {
			return err
		}

	default:
		if config.EnableSecureTLSBootstrapping || agent.IsTLSBootstrappingEnabledWithHardCodedToken(config.KubeletClientTLSBootstrapToken) {
			if err := useBootstrappingKubeConfig(config, files); err != nil {
				return err
			}
		} else {
			if err := useHardCodedKubeconfig(config, files); err != nil {
				return err
			}
		}
	}
	return nil
}

func useHardCodedKubeconfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	files[getHardCodedKubeconfigPath(config)] = File{
		Content: genContentKubeconfig(config),
		Mode:    ReadOnlyWorld,
	}
	return nil
}

func useArcTokenSh(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	bootstrapKubeconfig := genContentArcTokenSh(config)
	files[getArcTokenPath(config)] = File{
		Content: bootstrapKubeconfig,
		Mode:    ExecutableWorld,
	}
	return nil
}

func useAzureTokenSh(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	bootstrapKubeconfig := genContentAzureTokenSh(config)
	if config.AgentPoolProfile.IsWindows() {
		bootstrapKubeconfig = genContentAzureTokenPs1(config)
	}
	files[getAzureTokenPath(config)] = File{
		Content: bootstrapKubeconfig,
		Mode:    ExecutableWorld,
	}
	return nil
}

func useBootstrappingKubeConfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	bootstrapKubeconfig, err := genContentBootstrapKubeconfig(config)
	if err != nil {
		return fmt.Errorf("content bootstrap kubeconfig: %w", err)
	}

	files[getBootstrapKubeconfigPath(config)] = File{
		Content: bootstrapKubeconfig,
		Mode:    ReadOnlyWorld,
	}
	return nil
}

func genContentKubeconfig(config *datamodel.NodeBootstrappingConfiguration) string {
	var users string
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}

	switch config.AgentPoolProfile.BootstrappingMethod {
	case datamodel.UseArcMsiDirectly:
		users = fmt.Sprintf(`- name: default-auth
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: %s
      provideClusterInfo: false
`, getArcTokenPath(config))

	case datamodel.UseAzureMsiDirectly:
		if config.AgentPoolProfile.IsWindows() {
			users = `- name: default-auth
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: powershell
      args:
      - c:/k/azure-token.ps1
      provideClusterInfo: false
`
		} else {
			users = fmt.Sprintf(`- name: default-auth
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: %s
      provideClusterInfo: false
`, getAzureTokenPath(config))
		}
	default:
		users = `- name: client
  user:
    client-certificate: /etc/kubernetes/certs/client.crt
    client-key: /etc/kubernetes/certs/client.key`
	}

	return fmt.Sprintf(`
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: %s
    server: https://%s:443
users:
%s
contexts:
- context:
    cluster: localcluster
    user: client
  name: localclustercontext
current-context: localclustercontext
`, getCaCertPath(config), agent.GetKubernetesEndpoint(config.ContainerService), users)
}

func genContentArcTokenSh(config *datamodel.NodeBootstrappingConfiguration) string {
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}

	return fmt.Sprintf(`#!/bin/bash

# Fetch an AAD token from Azure Arc HIMDS and output it in the ExecCredential format
# https://learn.microsoft.com/azure/azure-arc/servers/managed-identity-authentication

TOKEN_URL="http://127.0.0.1:40342/metadata/identity/oauth2/token?api-version=2019-11-01&resource=%s"
EXECCREDENTIAL='''
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1",
  "spec": {
    "interactive": false
  },
  "status": {
    "expirationTimestamp": .expires_on | tonumber | todate,
    "token": .access_token
  }
}
'''

# Arc IMDS requires a challenge token from a file only readable by root for security
CHALLENGE_TOKEN_PATH=$(curl -s -D - -H Metadata:true $TOKEN_URL | grep Www-Authenticate | cut -d "=" -f 2 | tr -d "[:cntrl:]")
CHALLENGE_TOKEN=$(cat $CHALLENGE_TOKEN_PATH)
if [ $? -ne 0 ]; then
    echo "Could not retrieve challenge token, double check that this command is run with root privileges."
    exit 255
fi

curl -s -H Metadata:true -H "Authorization: Basic $CHALLENGE_TOKEN" $TOKEN_URL | jq "$EXECCREDENTIAL"
`, appID)
}

func genContentAzureTokenPs1(config *datamodel.NodeBootstrappingConfiguration) string {
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}
	clientID := config.AgentPoolProfile.BootstrappingManagedIdentityID

	return fmt.Sprintf(`C:\Users\tim\.azure-kubelogin\kubelogin get-token --environment AzurePublicCloud --server-id  %s --login msi --client-id %s`, appID, clientID)
}

func genContentAzureTokenSh(config *datamodel.NodeBootstrappingConfiguration) string {
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}

	return fmt.Sprintf(`#!/bin/bash

TOKEN_URL="http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=%s"
EXECCREDENTIAL='''
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1",
  "spec": {
    "interactive": false
  },
  "status": {
    "expirationTimestamp": .expires_on | tonumber | todate,
    "token": .access_token
  }
}
'''

curl -s -H Metadata:true $TOKEN_URL | jq "$EXECCREDENTIAL"
`, appID)
}

func genContentKubelet(config *datamodel.NodeBootstrappingConfiguration) string {
	data := make([][2]string, 0)
	data = append(data, [2]string{"KUBELET_FLAGS", agent.GetOrderedKubeletConfigFlagString(config)})
	data = append(data, [2]string{"KUBELET_REGISTER_SCHEDULABLE", "true"})
	data = append(data, [2]string{"NETWORK_POLICY", config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy})
	isKubernetesVersionGe := func(version string) bool {
		isKubernetes := config.ContainerService.Properties.OrchestratorProfile.IsKubernetes()
		isKubernetesVersionGe := agent.IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, version)
		return isKubernetes && isKubernetesVersionGe
	}

	if !isKubernetesVersionGe("1.17.0") {
		data = append(data, [2]string{"KUBELET_IMAGE", config.K8sComponents.HyperkubeImageURL})
	}

	labels := func() string {
		if isKubernetesVersionGe("1.16.0") {
			return agent.GetAgentKubernetesLabels(config.AgentPoolProfile, config)
		}
		return config.AgentPoolProfile.GetKubernetesLabels()
	}

	data = append(data, [2]string{"KUBELET_NODE_LABELS", labels()})
	if config.ContainerService.IsAKSCustomCloud() {
		data = append(data, [2]string{"AZURE_ENVIRONMENT_FILEPATH", "/etc/kubernetes/" + config.ContainerService.Properties.CustomCloudEnv.Name + ".json"})
	}

	result := ""
	for _, d := range data {
		result += fmt.Sprintf("%s=%s\n", d[0], d[1])
	}
	return result
}

func genContentBootstrapKubeconfig(config *datamodel.NodeBootstrappingConfiguration) (string, error) {
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}
	data := map[string]any{
		"apiVersion": "v1",
		"kind":       "Config",
		"clusters":   getContentKubletClusterInfo(config),
		"users": []map[string]any{
			{
				"name": "kubelet-bootstrap",
				"user": func() map[string]any {
					switch config.AgentPoolProfile.BootstrappingMethod {
					case datamodel.UseArcMsiToMakeCSR:
						m, done := getContentKubeletUserArcMsi(config)
						if done {
							return m
						}

					case datamodel.UseAzureMsiToMakeCSR:
						m, done := getContentKubletUserAzureMsi(config)
						if done {
							return m
						}
					}
					if config.EnableSecureTLSBootstrapping || config.AgentPoolProfile.BootstrappingMethod == datamodel.UseSecureTLSBootstrapping {
						return getContentKubeletUserSecureBootstrapping(appID)
					}
					return getContentKubeletUserBootstrapToken(config)
				}(),
			},
		},
		"contexts":        getContentKubeletContexts(),
		"current-context": "bootstrap-context",
	}
	dataYAML, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(dataYAML), nil
}

func getContentKubeletContexts() []map[string]any {
	return []map[string]any{
		{
			"context": map[string]any{
				"cluster": "localcluster",
				"user":    "kubelet-bootstrap",
			},
			"name": "bootstrap-context",
		},
	}
}

func getContentKubeletUserBootstrapToken(config *datamodel.NodeBootstrappingConfiguration) map[string]any {
	return map[string]any{
		"token": agent.GetTLSBootstrapTokenForKubeConfig(config.KubeletClientTLSBootstrapToken),
	}
}

func getContentKubeletUserSecureBootstrapping(appID string) map[string]any {
	return map[string]any{
		"exec": map[string]any{
			"apiVersion": "client.authentication.k8s.io/v1",
			"command":    "/opt/azure/tlsbootstrap/tls-bootstrap-client",
			"args": []string{
				"bootstrap",
				"--next-proto=aks-tls-bootstrap",
				"--aad-resource=" + appID},
			"interactiveMode":    "Never",
			"provideClusterInfo": true,
		},
	}
}

func getContentKubletUserAzureMsi(config *datamodel.NodeBootstrappingConfiguration) map[string]any {
	if config.AgentPoolProfile.IsWindows() {
		return map[string]any{
			"exec": map[string]any{
				"apiVersion":         "client.authentication.k8s.io/v1",
				"command":            "powershell",
				"args":               []string{"-C", getAzureTokenPath(config)},
				"interactiveMode":    "Never",
				"provideClusterInfo": false,
			},
		}
	} else {
		return map[string]any{
			"exec": map[string]any{
				"apiVersion":         "client.authentication.k8s.io/v1",
				"command":            getAzureTokenPath(config),
				"interactiveMode":    "Never",
				"provideClusterInfo": false,
			},
		}
	}
}

func getContentKubeletUserArcMsi(config *datamodel.NodeBootstrappingConfiguration) map[string]any {
	if config.AgentPoolProfile.IsWindows() {
		return map[string]any{
			"exec": map[string]any{
				"apiVersion":         "client.authentication.k8s.io/v1",
				"command":            "powershell",
				"args":               []string{getArcTokenPath(config)},
				"interactiveMode":    "Never",
				"provideClusterInfo": false,
			},
		}
	} else {
		return map[string]any{
			"exec": map[string]any{
				"apiVersion":         "client.authentication.k8s.io/v1",
				"command":            getArcTokenPath(config),
				"interactiveMode":    "Never",
				"provideClusterInfo": false,
			},
		}
	}
}

func getContentKubletClusterInfo(config *datamodel.NodeBootstrappingConfiguration) []map[string]any {
	return []map[string]any{
		{
			"name": "localcluster",
			"cluster": map[string]any{
				"certificate-authority": getCaCertPath(config),
				"server":                "https://" + agent.GetKubernetesEndpoint(config.ContainerService) + ":443",
			},
		},
	}
}

func getCaCertPath(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return "c:\\k\\ca.crt"
	}
	return "/etc/kubernetes/certs/ca.crt"
}

func getBootstrapKubeconfigPath(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return "c:\\k\\bootstrap-config"
	}
	return "/var/lib/kubelet/bootstrap-kubeconfig"
}

func getHardCodedKubeconfigPath(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return "c:\\k\\config"
	}
	return "/var/lib/kubelet/kubeconfig"
}

func getArcTokenPath(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return "c:\\k\\arc-token.ps1"
	}
	return "/opt/azure/bootstrap/arc-token.sh"
}

func getAzureTokenPath(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return "c:\\k\\azure-token.ps1"
	}
	return "/opt/azure/bootstrap/azure-token.sh"
}

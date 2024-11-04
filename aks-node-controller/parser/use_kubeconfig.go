package parser

import (
	"fmt"
	"os"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

const (
	DefaultAksAadAppID = "6dae42f8-4368-4678-94ff-3960e28e3630"
)

type File struct {
	Content string
	Mode    os.FileMode
}

func useKubeconfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	if err := useKubeConfig(config, files); err != nil {
		return err
	}
	switch config.KubenetesAuthMethod {
	case datamodel.UseArcMsiToMakeCSR,
		datamodel.UseArcMsiDirectly:
		if err := useArcTokenSh(config, files); err != nil {
			return err
		}

	case datamodel.UseAzureMsiDirectly,
		datamodel.UseAzureMsiToMakeCSR:
		if err := useAzureTokenSh(config, files); err != nil {
			return err
		}

	case datamodel.UseTLSBootstrapToken, datamodel.UseSecureTLSBootstrapping:
		break

	default:
		break
	}

	return nil
}

func useKubeConfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	kubeConfig, err := genContentKubeconfig(config)
	if err != nil {
		return err
	}
	if shouldKubeconfigBeBootstrapConfig(config) {
		files[getBootstrapKubeconfigPath(config)] = File{
			Content: kubeConfig,
			Mode:    ReadOnlyWorld,
		}
	} else {
		files[getHardCodedKubeconfigPath(config)] = File{
			Content: kubeConfig,
			Mode:    ReadOnlyWorld,
		}
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

func genContentKubeconfig(config *datamodel.NodeBootstrappingConfiguration) (string, error) {
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}
	userName := "client"
	context := "localclustercontext"
	if shouldKubeconfigBeBootstrapConfig(config) {
		userName = "kubelet-bootstrap"
		context = "bootstrap-context"
	}
	data := map[string]any{
		"apiVersion": "v1",
		"kind":       "Config",
		"clusters":   getContentKubletClusterInfo(config),
		"users": []map[string]any{
			{
				"name": userName,
				"user": func() map[string]any {
					switch config.KubenetesAuthMethod {
					case datamodel.UseArcMsiToMakeCSR, datamodel.UseArcMsiDirectly:
						return getContentKubeletUserArcMsi(config)

					case datamodel.UseAzureMsiToMakeCSR, datamodel.UseAzureMsiDirectly:
						return getContentKubletUserAzureMsi(config)

					case datamodel.UseSecureTLSBootstrapping:
						return getContentKubeletUserSecureBootstrapping(appID)

					case datamodel.UseTLSBootstrapToken:
						return getContentKubeletUserBootstrapToken(config)

					default:
						if config.EnableSecureTLSBootstrapping {
							return getContentKubeletUserSecureBootstrapping(appID)
						}
						if config.KubeletClientTLSBootstrapToken != nil && *config.KubeletClientTLSBootstrapToken != "" {
							return getContentKubeletUserBootstrapToken(config)
						}
						return map[string]any{
							"client-certificate": "/etc/kubernetes/certs/client.crt",
							"client-key":         "/etc/kubernetes/certs/client.key",
						}
					}
				}(),
			},
		},
		"contexts": []map[string]any{
			{
				"context": map[string]any{
					"cluster": "localcluster",
					"user":    userName,
				},
				"name": context,
			},
		},
		"current-context": context,
	}
	dataYAML, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(dataYAML), nil
}

func shouldKubeconfigBeBootstrapConfig(config *datamodel.NodeBootstrappingConfiguration) bool {
	return config.KubenetesAuthMethod == datamodel.UseSecureTLSBootstrapping ||
		config.KubeletClientTLSBootstrapToken != nil ||
		config.EnableSecureTLSBootstrapping ||
		config.KubenetesAuthMethod == datamodel.UseAzureMsiToMakeCSR ||
		config.KubenetesAuthMethod == datamodel.UseArcMsiToMakeCSR
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
	clientID := config.BootstrappingManagedIdentityID

	return fmt.Sprintf(`
$TOKEN_URL="http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=%s&client_id=%s"
$HEADERS = @{
 'Metadata' = 'true'
}

$RESULT = Invoke-WebRequest -Method GET -Headers $HEADERS -Uri $TOKEN_URL 
$CONTENT = $RESULT.Content | ConvertFrom-Json -Depth 4
$ACCESS_TOKEN = $CONTENT.access_token
$EXPIRES_ON = Get-Date -AsUTC -Format "o" (Get-Date 01.01.1970).AddSeconds($CONTENT.expires_on)

$EXECCREDENTIAL=@{
  'kind' = 'ExecCredential'
  'apiVersion' = "client.authentication.k8s.io/v1"
  'spec' = @{
    'interactive' = $False
  }
  'status' = @{
    'expirationTimestamp' = $EXPIRES_ON
    'token' = $ACCESS_TOKEN
  }
}

$EXECCREDENTIAL | ConvertTo-Json -Depth 4
`, appID, clientID)
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
				"command":            "pwsh",
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

package main

import (
	"fmt"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	bootstrapConfigFile = "/var/lib/kubelet/bootstrap-kubeconfig"
	kubeConfigFile      = "/var/lib/kubelet/kubeconfig"
	arcTokenSh          = "/opt/azure/bootstrap/arc-token.sh"
	azureTokenSh        = "/opt/azure/bootstrap/azure-token.sh"
)

func assertKubeconfig(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration, expected string) {
	t.Helper()
	files, err := customData(nbc)
	require.NoError(t, err)
	require.NotContains(t, files, bootstrapConfigFile)
	actual := getFile(t, nbc, kubeConfigFile, 0644)
	assert.YAMLEq(t, expected, actual)
}

func assertBootstrapKubeconfig(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration, expected string) {
	t.Helper()
	files, err := customData(nbc)
	require.NoError(t, err)
	require.NotContains(t, files, kubeConfigFile)
	actual := getFile(t, nbc, bootstrapConfigFile, 0644)
	assert.YAMLEq(t, expected, actual)
}

func assertArcTokenSh(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration, aadAppId string) {
	t.Helper()
	files, err := customData(nbc)
	require.NoError(t, err)
	require.NotContains(t, files, azureTokenSh)
	actual := getFile(t, nbc, arcTokenSh, 0644)
	expected := fmt.Sprintf(`#!/bin/bash

# Fetch an AAD token from Azure Arc HIMDS and output it in the ExecCredential format
# https://learn.microsoft.com/azure/azure-arc/servers/managed-identity-authentication

TOKEN_URL="http://127.0.0.1:40342/metadata/identity/oauth2/token?api-version=2019-11-01&resource=%s"
EXECCREDENTIAL='''
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "spec": {
    "interactive": false
  },
  "status": {
    "expirationTimestamp": .expires_on | tonumber | todate,
    "token": .access_token
  }
}
`, aadAppId)
	assert.Equal(t, expected, actual)
}

func TestKubeConfigGeneratedCorrectly(t *testing.T) {

	t.Run("kubeconfig", func(t *testing.T) {
		nbc := validNBC()
		assertKubeconfig(t, nbc, `
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://:443
users:
- name: client
  user:
    client-certificate: /etc/kubernetes/certs/client.crt
    client-key: /etc/kubernetes/certs/client.key
contexts:
- context:
    cluster: localcluster
    user: client
  name: localclustercontext
current-context: localclustercontext
`)
	})

	t.Run("bootstrap-kubeconfig", func(t *testing.T) {
		nbc := validNBC()
		nbc.KubeletClientTLSBootstrapToken = Ptr("test-token")
		assertBootstrapKubeconfig(t, nbc, `apiVersion: v1
clusters:
    - cluster:
        certificate-authority: /etc/kubernetes/certs/ca.crt
        server: https://:443
      name: localcluster
contexts:
    - context:
        cluster: localcluster
        user: kubelet-bootstrap
      name: bootstrap-context
current-context: bootstrap-context
kind: Config
users:
    - name: kubelet-bootstrap
      user:
        token: test-token
`)
	})

	t.Run("secureTlsBootstrapKubeConfig sets bootstrap-kubeconfig correctly", func(t *testing.T) {
		nbc := validNBC()
		nbc.EnableSecureTLSBootstrapping = true
		assertBootstrapKubeconfig(t, nbc, `apiVersion: v1
clusters:
    - cluster:
        certificate-authority: /etc/kubernetes/certs/ca.crt
        server: https://:443
      name: localcluster
contexts:
    - context:
        cluster: localcluster
        user: kubelet-bootstrap
      name: bootstrap-context
current-context: bootstrap-context
kind: Config
users:
- name: kubelet-bootstrap
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      args:
        - bootstrap
        - --next-proto=aks-tls-bootstrap
        - --aad-resource=test-app-id
      command: /opt/azure/tlsbootstrap/tls-bootstrap-client
      interactiveMode: Never
      provideClusterInfo: true
`)
	})

	t.Run("BootstrappingMethod=UseSecureTlsBootstrapping sets bootstrap-kubeconfig correctly", func(t *testing.T) {
		nbc := validNBC()
		nbc.BootstrappingMethod = "UseSecureTlsBootstrapping"
		assertBootstrapKubeconfig(t, nbc, `apiVersion: v1
clusters:
    - cluster:
        certificate-authority: /etc/kubernetes/certs/ca.crt
        server: https://:443
      name: localcluster
contexts:
    - context:
        cluster: localcluster
        user: kubelet-bootstrap
      name: bootstrap-context
current-context: bootstrap-context
kind: Config
users:
- name: kubelet-bootstrap
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      args:
        - bootstrap
        - --next-proto=aks-tls-bootstrap
        - --aad-resource=test-app-id
      command: /opt/azure/tlsbootstrap/tls-bootstrap-client
      interactiveMode: Never
      provideClusterInfo: true
`)
	})

	t.Run("BootstrappingMethod=UseTlsBootstrapToken sets bootstrap-kubeconfig correctly", func(t *testing.T) {
		nbc := validNBC()
		nbc.BootstrappingMethod = "UseTlsBootstrapToken"
		nbc.KubeletClientTLSBootstrapToken = Ptr("test-token-value")
		assertBootstrapKubeconfig(t, nbc, `apiVersion: v1
clusters:
    - cluster:
        certificate-authority: /etc/kubernetes/certs/ca.crt
        server: https://:443
      name: localcluster
contexts:
    - context:
        cluster: localcluster
        user: kubelet-bootstrap
      name: bootstrap-context
current-context: bootstrap-context
kind: Config
users:
    - name: kubelet-bootstrap
      user:
        token: test-token-value
`)
	})

	t.Run("BootstrappingMethod=UseArcMsiToMakeCSR sets bootstrap-kubeconfig correctly", func(t *testing.T) {
		nbc := validNBC()
		nbc.BootstrappingMethod = "UseArcMsiToMakeCSR"
		assertBootstrapKubeconfig(t, nbc, `apiVersion: v1
clusters:
    - cluster:
        certificate-authority: /etc/kubernetes/certs/ca.crt
        server: https://:443
      name: localcluster
contexts:
    - context:
        cluster: localcluster
        user: kubelet-bootstrap
      name: bootstrap-context
current-context: bootstrap-context
kind: Config
users:
    - name: kubelet-bootstrap
      user:
        exec:
          apiVersion: client.authentication.k8s.io/v1
          command: /opt/azure/bootstrap/arc-token.sh
          provideClusterInfo: false
`)
	})

	t.Run("BootstrappingMethod=UseArcMsiToMakeCSR sets token.sh correctly with the AKS AAD App ID", func(t *testing.T) {
		nbc := validNBC()
		nbc.CustomSecureTLSBootstrapAADServerAppID = ""
		nbc.BootstrappingMethod = "UseArcMsiToMakeCSR"
		assertArcTokenSh(t, nbc, "6dae42f8-4368-4678-94ff-3960e28e3630")
	})

	t.Run("BootstrappingMethod=UseArcMsiToMakeCSR sets token.sh correctly with a different AKS AAD App ID", func(t *testing.T) {
		nbc := validNBC()
		nbc.CustomSecureTLSBootstrapAADServerAppID = "different_app_id"
		nbc.BootstrappingMethod = "UseArcMsiToMakeCSR"
		assertArcTokenSh(t, nbc, "different_app_id")
	})

}

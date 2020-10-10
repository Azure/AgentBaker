// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/aks-engine/helpers"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"

	_ "k8s.io/client-go/plugin/pkg/client/auth/azure" // register azure (AD) authentication plugin
)

var k8sKubeconfigJson = []byte(`    {
	"apiVersion": "v1",
	"clusters": [
		{
			"cluster": {
				"certificate-authority-data": "{{WrapAsVerbatim "parameters('caCertificate')"}}",
				"server": "https://{{WrapAsVerbatim "reference(concat('Microsoft.Network/publicIPAddresses/', variables('masterPublicIPAddressName'))).dnsSettings.fqdn"}}"
			},
			"name": "{{WrapAsVariable "resourceGroup"}}"
		}
	],
	"contexts": [
		{
			"context": {
				"cluster": "{{WrapAsVariable "resourceGroup"}}",
				"user": "{{WrapAsVariable "resourceGroup"}}-admin"
			},
			"name": "{{WrapAsVariable "resourceGroup"}}"
		}
	],
	"current-context": "{{WrapAsVariable "resourceGroup"}}",
	"kind": "Config",
	"users": [
		{
			"name": "{{WrapAsVariable "resourceGroup"}}-admin",
			"user": {{authInfo}}
		}
	]
}
`)

const (
	// DefaultInternalLbStaticIPOffset specifies the offset of the internal LoadBalancer's IP
	// address relative to the first consecutive Kubernetes static IP
	DefaultInternalLbStaticIPOffset = 10
)

// GenerateKubeConfig returns a JSON string representing the KubeConfig
func GenerateKubeConfig(properties *datamodel.Properties, location string, cloudSpecConfig *datamodel.AzureEnvironmentSpecConfig) (string, error) {
	if properties == nil {
		return "", errors.New("Properties nil in GenerateKubeConfig")
	}
	if properties.CertificateProfile == nil {
		return "", errors.New("CertificateProfile property may not be nil in GenerateKubeConfig")
	}
	kubeconfig := string(k8sKubeconfigJson)
	// variable replacement
	kubeconfig = strings.Replace(kubeconfig, "{{WrapAsVerbatim \"parameters('caCertificate')\"}}", base64.StdEncoding.EncodeToString([]byte(properties.CertificateProfile.CaCertificate)), -1)
	if !(properties.OrchestratorProfile != nil &&
		properties.OrchestratorProfile.KubernetesConfig != nil &&
		properties.OrchestratorProfile.KubernetesConfig.PrivateCluster != nil &&
		to.Bool(properties.OrchestratorProfile.KubernetesConfig.PrivateCluster.Enabled)) {
		kubeconfig = strings.Replace(kubeconfig, "{{WrapAsVerbatim \"reference(concat('Microsoft.Network/publicIPAddresses/', variables('masterPublicIPAddressName'))).dnsSettings.fqdn\"}}", datamodel.FormatProdFQDNByLocation(properties.MasterProfile.DNSPrefix, location, cloudSpecConfig), -1)
	}
	kubeconfig = strings.Replace(kubeconfig, "{{WrapAsVariable \"resourceGroup\"}}", properties.MasterProfile.DNSPrefix, -1)

	var authInfo string
	if properties.AADProfile == nil {
		authInfo = fmt.Sprintf("{\"client-certificate-data\":\"%v\",\"client-key-data\":\"%v\"}",
			base64.StdEncoding.EncodeToString([]byte(properties.CertificateProfile.KubeConfigCertificate)),
			base64.StdEncoding.EncodeToString([]byte(properties.CertificateProfile.KubeConfigPrivateKey)))
	} else {
		tenantID := properties.AADProfile.TenantID
		if len(tenantID) == 0 {
			tenantID = "common"
		}

		authInfo = fmt.Sprintf("{\"auth-provider\":{\"name\":\"azure\",\"config\":{\"environment\":\"%v\",\"tenant-id\":\"%v\",\"apiserver-id\":\"%v\",\"client-id\":\"%v\"}}}",
			helpers.GetTargetEnv(location, properties.GetCustomCloudName()),
			tenantID,
			properties.AADProfile.ServerAppID,
			properties.AADProfile.ClientAppID)
	}
	kubeconfig = strings.Replace(kubeconfig, "{{authInfo}}", authInfo, -1)

	return kubeconfig, nil
}

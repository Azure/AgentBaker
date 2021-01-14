// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"fmt"
	"path"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/aks-engine/api"
	"github.com/Azure/agentbaker/pkg/aks-engine/helpers"
)

// ArtifactWriter represents the object that writes artifacts
type ArtifactWriter struct{}

// WriteTLSArtifacts saves TLS certificates and keys to the server filesystem
func (w *ArtifactWriter) WriteTLSArtifacts(containerService *datamodel.ContainerService, apiVersion, template, parameters, artifactsDir string, certsGenerated bool, parametersOnly bool,
	cloudSpecConfig *datamodel.AzureEnvironmentSpecConfig) error {
	if len(artifactsDir) == 0 {
		artifactsDir = fmt.Sprintf("%s-%s", containerService.Properties.OrchestratorProfile.OrchestratorType, containerService.Properties.GetClusterID())
		artifactsDir = path.Join("_output", artifactsDir)
	}

	f := &helpers.FileSaver{}

	// convert back the API object, and write it
	var b []byte
	var err error
	if !parametersOnly {
		apiloader := &api.Apiloader{}
		b, err = apiloader.SerializeContainerService(containerService, apiVersion)

		if err != nil {
			return err
		}

		if e := f.SaveFile(artifactsDir, "apimodel.json", b); e != nil {
			return e
		}

		if e := f.SaveFileString(artifactsDir, "azuredeploy.json", template); e != nil {
			return e
		}
	}

	if e := f.SaveFileString(artifactsDir, "azuredeploy.parameters.json", parameters); e != nil {
		return e
	}

	if !certsGenerated {
		return nil
	}

	properties := containerService.Properties
	if properties.OrchestratorProfile.IsKubernetes() {
		directory := path.Join(artifactsDir, "kubeconfig")
		var locations []string
		if containerService.Location != "" {
			locations = []string{containerService.Location}
		} else {
			locations = helpers.GetAzureLocations()
		}

		for _, location := range locations {
			b, gkcerr := GenerateKubeConfig(properties, location, cloudSpecConfig)
			if gkcerr != nil {
				return gkcerr
			}
			if e := f.SaveFileString(directory, fmt.Sprintf("kubeconfig.%s.json", location), b); e != nil {
				return e
			}
		}

		if e := f.SaveFileString(artifactsDir, "ca.key", properties.CertificateProfile.CaPrivateKey); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "ca.crt", properties.CertificateProfile.CaCertificate); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "apiserver.key", properties.CertificateProfile.APIServerPrivateKey); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "apiserver.crt", properties.CertificateProfile.APIServerCertificate); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "client.key", properties.CertificateProfile.ClientPrivateKey); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "client.crt", properties.CertificateProfile.ClientCertificate); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "kubectlClient.key", properties.CertificateProfile.KubeConfigPrivateKey); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "kubectlClient.crt", properties.CertificateProfile.KubeConfigCertificate); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "etcdserver.key", properties.CertificateProfile.EtcdServerPrivateKey); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "etcdserver.crt", properties.CertificateProfile.EtcdServerCertificate); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "etcdclient.key", properties.CertificateProfile.EtcdClientPrivateKey); e != nil {
			return e
		}
		if e := f.SaveFileString(artifactsDir, "etcdclient.crt", properties.CertificateProfile.EtcdClientCertificate); e != nil {
			return e
		}
	}

	return nil
}

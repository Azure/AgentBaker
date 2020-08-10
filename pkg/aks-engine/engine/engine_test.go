// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"path"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"

	"github.com/leonelquinteros/gotext"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/i18n"
)

func TestGenerateKubeConfig(t *testing.T) {
	locale := gotext.NewLocale(path.Join("..", "..", "translations"), "en_US")
	i18n.Initialize(locale)

	apiloader := &api.Apiloader{
		Translator: &i18n.Translator{
			Locale: locale,
		},
	}

	testData := "./testdata/simple/kubernetes.json"

	containerService, _, err := apiloader.LoadContainerServiceFromFile(testData, true, false, nil)
	if err != nil {
		t.Errorf("Failed to load container service from file: %v", err)
	}
	kubeConfig, err := GenerateKubeConfig(containerService.Properties, "westus2")
	// TODO add actual kubeconfig validation
	if len(kubeConfig) < 1 {
		t.Errorf("Got unexpected kubeconfig payload: %v", kubeConfig)
	}
	if err != nil {
		t.Errorf("Failed to call GenerateKubeConfig with simple Kubernetes config from file: %v", testData)
	}

	p := api.Properties{}
	_, err = GenerateKubeConfig(&p, "westus2")
	if err == nil {
		t.Errorf("Expected an error result from nil Properties child properties")
	}

	_, err = GenerateKubeConfig(nil, "westus2")
	if err == nil {
		t.Errorf("Expected an error result from nil Properties child properties")
	}

	containerService.Properties.OrchestratorProfile.KubernetesConfig.PrivateCluster = &api.PrivateCluster{
		Enabled: to.BoolPtr(true),
	}

	_, err = GenerateKubeConfig(containerService.Properties, "westus2")
	if err != nil {
		t.Errorf("Failed to call GenerateKubeConfig with simple Kubernetes config from file: %v", testData)
	}

	containerService.Properties.MasterProfile.Count = 3
	_, err = GenerateKubeConfig(containerService.Properties, "westus2")
	if err == nil {
		t.Errorf("expected an error result when Private Cluster is Enabled and no FirstConsecutiveStaticIP was specified")
	}

	containerService.Properties.MasterProfile.FirstConsecutiveStaticIP = "10.239.255.239"
	_, err = GenerateKubeConfig(containerService.Properties, "westus2")
	if err != nil {
		t.Errorf("Failed to call GenerateKubeConfig with simple Kubernetes config from file: %v", testData)
	}

	containerService.Properties.AADProfile = &api.AADProfile{
		ClientAppID: "fooClientAppID",
		TenantID:    "fooTenantID",
		ServerAppID: "fooServerAppID",
	}

	_, err = GenerateKubeConfig(containerService.Properties, "westus2")
	if err != nil {
		t.Errorf("Failed to call GenerateKubeConfig with simple Kubernetes config from file: %v", testData)
	}
}

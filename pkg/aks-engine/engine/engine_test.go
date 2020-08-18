// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package engine

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	aksenginefork "github.com/Azure/agentbaker/pkg/aks-engine/api"
	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/go-autorest/autorest/to"
)

func TestGenerateKubeConfig(t *testing.T) {
	apiloader := &aksenginefork.Apiloader{}
	testData := "./testdata/simple/kubernetes.json"

	containerService, _, err := apiloader.LoadContainerServiceFromFile(testData)
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

	p := datamodel.Properties{}
	_, err = GenerateKubeConfig(&p, "westus2")
	if err == nil {
		t.Errorf("Expected an error result from nil Properties child properties")
	}

	_, err = GenerateKubeConfig(nil, "westus2")
	if err == nil {
		t.Errorf("Expected an error result from nil Properties child properties")
	}

	containerService.Properties.OrchestratorProfile = &datamodel.OrchestratorProfile{
		KubernetesConfig: &datamodel.KubernetesConfig{},
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

	containerService.Properties.AADProfile = &datamodel.AADProfile{
		ClientAppID: "fooClientAppID",
		TenantID:    "fooTenantID",
		ServerAppID: "fooServerAppID",
	}

	_, err = GenerateKubeConfig(containerService.Properties, "westus2")
	if err != nil {
		t.Errorf("Failed to call GenerateKubeConfig with simple Kubernetes config from file: %v", testData)
	}
}

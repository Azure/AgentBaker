// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"strings"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/helpers"
)

// ContainerService complies with the ARM model of
// resource definition in a JSON template.
type ContainerService struct {
	ID       string                    `json:"id"`
	Location string                    `json:"location"`
	Name     string                    `json:"name"`
	Plan     *api.ResourcePurchasePlan `json:"plan,omitempty"`
	Tags     map[string]string         `json:"tags"`
	Type     string                    `json:"type"`

	Properties *api.Properties `json:"properties,omitempty"`
}

// GetCloudSpecConfig returns the Kubernetes container images URL configurations based on the deploy target environment.
//for example: if the target is the public azure, then the default container image url should be k8s.gcr.io/...
//if the target is azure china, then the default container image should be mirror.azure.cn:5000/google_container/...
func (cs *ContainerService) GetCloudSpecConfig() api.AzureEnvironmentSpecConfig {
	targetEnv := helpers.GetTargetEnv(cs.Location, cs.Properties.GetCustomCloudName())
	return api.AzureCloudSpecEnvMap[targetEnv]
}

// IsAKSCustomCloud checks if it's in AKS custom cloud
func (cs *ContainerService) IsAKSCustomCloud() bool {
	return cs.Properties.CustomCloudEnv != nil &&
		strings.EqualFold(cs.Properties.CustomCloudEnv.Name, "akscustom")
}

// FromAksEngineContainerService converts aks-engine's ContainerService to our
// own ContainerService. This is temporarily needed while we are still using
// aks-engine's ApiLoader to load ContainerService from file. Once that code
// is ported into our code base, we'll be using our own datamodel consistently
// through out the code base and this conversion function won't be needed.
func FromAksEngineContainerService(aksEngineCS *api.ContainerService) *ContainerService {
	return &ContainerService{
		ID:         aksEngineCS.ID,
		Location:   aksEngineCS.Location,
		Name:       aksEngineCS.Name,
		Plan:       aksEngineCS.Plan,
		Tags:       aksEngineCS.Tags,
		Type:       aksEngineCS.Type,
		Properties: aksEngineCS.Properties,
	}
}

// ToAksEngineContainerService converts our ContainerService to aks-engine's
// ContainerService to our. This is temporarily needed until we have finished
// porting all aks-engine code that's used by us into our own code base.
func ToAksEngineContainerService(cs *ContainerService) *api.ContainerService {
	return &api.ContainerService{
		ID:         cs.ID,
		Location:   cs.Location,
		Name:       cs.Name,
		Plan:       cs.Plan,
		Tags:       cs.Tags,
		Type:       cs.Type,
		Properties: cs.Properties,
	}
}

// GetLocations returns all supported regions.
// If AzureStackCloud, GetLocations provides the location of container service
// If AzurePublicCloud, AzureChinaCloud,AzureGermanCloud or AzureUSGovernmentCloud, GetLocations provides all azure regions in prod.
func (cs *ContainerService) GetLocations() []string {
	var allLocations []string
	if cs.Properties.IsAzureStackCloud() {
		allLocations = []string{cs.Location}
	} else {
		allLocations = helpers.GetAzureLocations()
	}
	return allLocations
}

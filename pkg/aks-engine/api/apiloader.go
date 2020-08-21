// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/aks-engine/helpers"
	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/api/vlabs"
)

const (
	defaultOrchestrator  = api.Kubernetes
	defaultAPIVersion    = vlabs.APIVersion
	defaultMasterCount   = 3
	defaultVMSize        = "Standard_DS2_v2"
	defaultOSDiskSizeGB  = 200
	defaultAgentPoolName = "agent"
	defaultAgentCount    = 3
	defaultAdminUser     = "azureuser"
)

// Apiloader represents the object that loads api model
type Apiloader struct{}

// LoadContainerServiceFromFile loads an AKS Cluster API Model from a JSON file
func (a *Apiloader) LoadContainerServiceFromFile(jsonFile string) (*datamodel.ContainerService, string, error) {
	contents, e := ioutil.ReadFile(jsonFile)
	if e != nil {
		return nil, "", fmt.Errorf("error reading file %s: %s", jsonFile, e.Error())
	}
	return a.DeserializeContainerService(contents)
}

// DeserializeContainerService loads an AKS Engine Cluster API Model, validates it, and returns the unversioned representation
func (a *Apiloader) DeserializeContainerService(contents []byte) (*datamodel.ContainerService, string, error) {
	m := &api.TypeMeta{}
	if err := json.Unmarshal(contents, &m); err != nil {
		return nil, "", err
	}

	cs, err := a.LoadContainerService(contents)
	return cs, m.APIVersion, err
}

func (a *Apiloader) LoadContainerService(
	contents []byte) (*datamodel.ContainerService, error) {

	containerService := &datamodel.ContainerService{}
	if e := json.Unmarshal(contents, &containerService); e != nil {
		return nil, e
	}
	if e := checkJSONKeys(contents, reflect.TypeOf(*containerService), reflect.TypeOf(api.TypeMeta{})); e != nil {
		return nil, e
	}
	return containerService, nil
}

// VlabsARMContainerService is the type we read and write from file
// needed because the json that is sent to ARM and aks-engine
// is different from the json that the ACS RP Api gets from ARM
//
// This was copied from aks-engine's github.com/Azure/aks-engine/pkg/api/types.go
type vlabsARMContainerService struct {
	api.TypeMeta
	*datamodel.ContainerService
}

// SerializeContainerService takes an unversioned container service and returns the bytes
func (a *Apiloader) SerializeContainerService(containerService *datamodel.ContainerService, version string) ([]byte, error) {
	switch version {
	case vlabs.APIVersion:
		armContainerService := &vlabsARMContainerService{}
		armContainerService.ContainerService = containerService
		armContainerService.APIVersion = version
		b, err := helpers.JSONMarshalIndent(armContainerService, "", "  ", false)
		if err != nil {
			return nil, err
		}
		return b, nil

	default:
		return nil, fmt.Errorf("invalid version %s for conversion back from unversioned object", version)
	}
}

// LoadAgentpoolProfileFromFile loads an an AgentPoolProfile object from a JSON file
func (a *Apiloader) LoadAgentpoolProfileFromFile(jsonFile string) (*datamodel.AgentPoolProfile, error) {
	contents, e := ioutil.ReadFile(jsonFile)
	if e != nil {
		return nil, fmt.Errorf("error reading file %s: %s", jsonFile, e.Error())
	}
	return a.LoadAgentPoolProfile(contents)
}

// LoadAgentPoolProfile marshalls raw data into a strongly typed AgentPoolProfile return object
func (a *Apiloader) LoadAgentPoolProfile(contents []byte) (*datamodel.AgentPoolProfile, error) {
	agentPoolProfile := &datamodel.AgentPoolProfile{}
	if e := json.Unmarshal(contents, &agentPoolProfile); e != nil {
		return nil, e
	}
	if e := checkJSONKeys(contents, reflect.TypeOf(*agentPoolProfile), reflect.TypeOf(api.TypeMeta{})); e != nil {
		return nil, e
	}
	return agentPoolProfile, nil
}

// LoadDefaultContainerServiceProperties loads the default API model
func LoadDefaultContainerServiceProperties() (api.TypeMeta, *vlabs.Properties) {
	return api.TypeMeta{APIVersion: defaultAPIVersion}, &vlabs.Properties{
		OrchestratorProfile: &vlabs.OrchestratorProfile{
			OrchestratorType: defaultOrchestrator,
		},
		MasterProfile: &vlabs.MasterProfile{
			Count:        defaultMasterCount,
			VMSize:       defaultVMSize,
			OSDiskSizeGB: defaultOSDiskSizeGB,
		},
		AgentPoolProfiles: []*vlabs.AgentPoolProfile{
			{
				Name:         defaultAgentPoolName,
				Count:        defaultAgentCount,
				VMSize:       defaultVMSize,
				OSDiskSizeGB: defaultOSDiskSizeGB,
			},
		},
		LinuxProfile: &vlabs.LinuxProfile{AdminUsername: defaultAdminUser},
	}
}

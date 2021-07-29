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
)

const (
	VlabsAPIVersion      = "vlabs"
	defaultOrchestrator  = datamodel.Kubernetes
	defaultAPIVersion    = VlabsAPIVersion
	defaultMasterCount   = 3
	defaultVMSize        = "Standard_DS2_v2"
	defaultAgentPoolName = "agent"
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
	m := &datamodel.TypeMeta{}
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
	if e := checkJSONKeys(contents, reflect.TypeOf(*containerService), reflect.TypeOf(datamodel.TypeMeta{})); e != nil {
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
	datamodel.TypeMeta
	*datamodel.ContainerService
}

// SerializeContainerService takes an unversioned container service and returns the bytes
func (a *Apiloader) SerializeContainerService(containerService *datamodel.ContainerService, version string) ([]byte, error) {
	switch version {
	case VlabsAPIVersion:
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

// LoadDefaultContainerServiceProperties loads the default API model
func LoadDefaultContainerServiceProperties() (datamodel.TypeMeta, *datamodel.Properties) {
	return datamodel.TypeMeta{APIVersion: defaultAPIVersion}, &datamodel.Properties{
		OrchestratorProfile: &datamodel.OrchestratorProfile{
			OrchestratorType: defaultOrchestrator,
		},
		HostedMasterProfile: &datamodel.HostedMasterProfile{},
		AgentPoolProfiles: []*datamodel.AgentPoolProfile{
			{
				Name:   defaultAgentPoolName,
				VMSize: defaultVMSize,
			},
		},
		LinuxProfile: &datamodel.LinuxProfile{AdminUsername: defaultAdminUser},
	}
}

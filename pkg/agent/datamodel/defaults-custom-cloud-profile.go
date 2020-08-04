// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/aks-engine/pkg/api"
	"github.com/Azure/aks-engine/pkg/helpers"
	"github.com/Azure/go-autorest/autorest/azure"
)

func (cs *ContainerService) setCustomCloudProfileDefaults(params api.CustomCloudProfileDefaultsParams) error {
	p := cs.Properties
	if p.IsAzureStackCloud() {
		p.CustomCloudProfile.AuthenticationMethod = helpers.EnsureString(p.CustomCloudProfile.AuthenticationMethod, api.ClientSecretAuthMethod)
		p.CustomCloudProfile.IdentitySystem = helpers.EnsureString(p.CustomCloudProfile.IdentitySystem, api.AzureADIdentitySystem)
		p.CustomCloudProfile.DependenciesLocation = api.DependenciesLocation(helpers.EnsureString(string(p.CustomCloudProfile.DependenciesLocation), api.AzureStackDependenciesLocationPublic))
		err := cs.SetCustomCloudProfileEnvironment()
		if err != nil {
			return fmt.Errorf("Failed to set environment - %s", err)
		}
		err = p.SetAzureStackCloudSpec(api.AzureStackCloudSpecParams(params))
		if err != nil {
			return fmt.Errorf("Failed to set cloud spec - %s", err)
		}
	}
	return nil
}

// SetCustomCloudProfileEnvironment retrieves the endpoints from Azure Stack metadata endpoint and sets the values for azure.Environment
func (cs *ContainerService) SetCustomCloudProfileEnvironment() error {
	p := cs.Properties
	if p.IsAzureStackCloud() {
		if p.CustomCloudProfile.Environment == nil {
			p.CustomCloudProfile.Environment = &azure.Environment{}
		}

		env := p.CustomCloudProfile.Environment
		if env.Name == "" || env.ResourceManagerEndpoint == "" || env.ServiceManagementEndpoint == "" || env.ActiveDirectoryEndpoint == "" || env.GraphEndpoint == "" || env.ResourceManagerVMDNSSuffix == "" {
			env.Name = api.AzureStackCloud
			if !strings.HasPrefix(p.CustomCloudProfile.PortalURL, fmt.Sprintf("https://portal.%s.", cs.Location)) {
				return fmt.Errorf("portalURL needs to start with https://portal.%s. ", cs.Location)
			}
			azsFQDNSuffix := strings.Replace(p.CustomCloudProfile.PortalURL, fmt.Sprintf("https://portal.%s.", cs.Location), "", -1)
			azsFQDNSuffix = strings.TrimSuffix(azsFQDNSuffix, "/")
			env.ResourceManagerEndpoint = fmt.Sprintf("https://management.%s.%s/", cs.Location, azsFQDNSuffix)
			metadataURL := fmt.Sprintf("%s/metadata/endpoints?api-version=1.0", strings.TrimSuffix(env.ResourceManagerEndpoint, "/"))

			// Retrieve the metadata
			httpClient := &http.Client{
				Timeout: 30 * time.Second,
			}
			endpointsresp, err := httpClient.Get(metadataURL)
			if err != nil || endpointsresp.StatusCode != 200 {
				return fmt.Errorf("%s . apimodel invalid: failed to retrieve Azure Stack endpoints from %s", err, metadataURL)
			}

			body, err := ioutil.ReadAll(endpointsresp.Body)
			if err != nil {
				return fmt.Errorf("%s . apimodel invalid: failed to read the response from %s", err, metadataURL)
			}

			endpoints := api.AzureStackMetadataEndpoints{}
			err = json.Unmarshal(body, &endpoints)
			if err != nil {
				return fmt.Errorf("%s . apimodel invalid: failed to parse the response from %s", err, metadataURL)
			}

			if endpoints.GraphEndpoint == "" || endpoints.Authentication == nil || endpoints.Authentication.LoginEndpoint == "" || len(endpoints.Authentication.Audiences) == 0 || endpoints.Authentication.Audiences[0] == "" {
				return fmt.Errorf("%s . apimodel invalid: invalid response from %s", err, metadataURL)
			}

			env.GraphEndpoint = endpoints.GraphEndpoint
			env.ServiceManagementEndpoint = endpoints.Authentication.Audiences[0]
			env.GalleryEndpoint = endpoints.GalleryEndpoint
			env.ActiveDirectoryEndpoint = endpoints.Authentication.LoginEndpoint
			if p.CustomCloudProfile.IdentitySystem == api.ADFSIdentitySystem {
				env.ActiveDirectoryEndpoint = strings.TrimSuffix(env.ActiveDirectoryEndpoint, "/")
				env.ActiveDirectoryEndpoint = strings.TrimSuffix(env.ActiveDirectoryEndpoint, "adfs")
			}

			env.ManagementPortalURL = endpoints.PortalEndpoint
			env.ResourceManagerVMDNSSuffix = fmt.Sprintf("cloudapp.%s", azsFQDNSuffix)
			env.StorageEndpointSuffix = fmt.Sprintf("%s.%s", cs.Location, azsFQDNSuffix)
			env.KeyVaultDNSSuffix = fmt.Sprintf("vault.%s.%s", cs.Location, azsFQDNSuffix)
		}
	}
	return nil
}

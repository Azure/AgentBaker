package aksnodeconfigv1

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// AKSNodeConfigBuilder is a helper struct to build the AKSNodeConfig.
// It provides methods to apply configuration, get the AKSNodeConfig object, and validate the contract, etc.
type AKSNodeConfigBuilder struct {
	// aKSNodeConfig is the configuration object for the AKSNodeConfig.
	aKSNodeConfig *Configuration
}

// Check and initialize each field if it is nil.
func initializeIfNil[T any](field **T) {
	if *field == nil {
		*field = new(T)
	}
}

// Ensure all objects are non-nil. Please add new objects here.
func ensureConfigsNonNil(nBC *Configuration) {
	initializeIfNil(&nBC.KubeBinaryConfig)
	initializeIfNil(&nBC.ApiServerConfig)
	initializeIfNil(&nBC.AuthConfig)
	initializeIfNil(&nBC.ClusterConfig)
	initializeIfNil(&nBC.ClusterConfig.LoadBalancerConfig)
	initializeIfNil(&nBC.ClusterConfig.ClusterNetworkConfig)
	initializeIfNil(&nBC.GpuConfig)
	initializeIfNil(&nBC.NetworkConfig)
	initializeIfNil(&nBC.TlsBootstrappingConfig)
	initializeIfNil(&nBC.KubeletConfig)
	initializeIfNil(&nBC.RuncConfig)
	initializeIfNil(&nBC.ContainerdConfig)
	initializeIfNil(&nBC.TeleportConfig)
	initializeIfNil(&nBC.CustomLinuxOsConfig)
	initializeIfNil(&nBC.CustomLinuxOsConfig.SysctlConfig)
	initializeIfNil(&nBC.CustomLinuxOsConfig.UlimitConfig)
	initializeIfNil(&nBC.HttpProxyConfig)
	initializeIfNil(&nBC.CustomCloudConfig)
	initializeIfNil(&nBC.CustomSearchDomainConfig)
}

// NewAKSNodeConfigBuilder creates a new instance of AKSNodeConfigBuilder and ensures all objects in aKSNodeConfig are non-nil.
func NewAKSNodeConfigBuilder() *AKSNodeConfigBuilder {
	config := Configuration{
		Version: contractVersion,
	}
	ensureConfigsNonNil(&config)
	aKSNodeConfig := &AKSNodeConfigBuilder{aKSNodeConfig: &config}
	return aKSNodeConfig
}

// ApplyConfiguration Applies the configuration to the aKSNodeConfig object.
func (aKSNodeConfigBuilder *AKSNodeConfigBuilder) ApplyConfiguration(config *Configuration) {
	if config == nil {
		return
	}

	// Use deep copy to avoid modifying the original object 'config'.
	if err := deepCopy(config, aKSNodeConfigBuilder.aKSNodeConfig); err != nil {
		log.Printf("Failed to deep copy the configuration: %v", err)
		ensureConfigsNonNil(aKSNodeConfigBuilder.aKSNodeConfig)
	}
}

// GetAKSNodeConfig gets the aKSNodeConfig object.
func (aKSNodeConfigBuilder *AKSNodeConfigBuilder) GetAKSNodeConfig() *Configuration {
	return aKSNodeConfigBuilder.aKSNodeConfig
}

// Deep copy the source object to the destination object.
// Note that the existing value in the destination object will not be cleared
// if the source object doesn't have that field.
func deepCopy(src, dst interface{}) error {
	if src == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	return gob.NewDecoder(&buf).Decode(dst)
}

// ValidateAKSNodeConfig validates the AKSNodeConfig.
// It returns an error if the contract is invalid.
// This function should be called after applying all configuration and before sending to downstream component.
func (aKSNodeConfigBuilder *AKSNodeConfigBuilder) ValidateAKSNodeConfig() error {
	if err := aKSNodeConfigBuilder.validateRequiredFields(); err != nil {
		return err
	}
	// Add more validations here if needed.

	return nil
}

// GetNodeBootstrapping gets the NodeBootstrapping object.
func (aKSNodeConfigBuilder *AKSNodeConfigBuilder) GetNodeBootstrapping() (*datamodel.NodeBootstrapping, error) {
	scriptlessCustomData, err := getScriptlessCustomDataContent(aKSNodeConfigBuilder.aKSNodeConfig)
	if err != nil {
		return nil, err
	}

	nodeBootstrapping := &datamodel.NodeBootstrapping{
		CSE:        scriptlessBootstrapStatusCSE,
		CustomData: scriptlessCustomData,
	}

	return nodeBootstrapping, nil
}

func getScriptlessCustomDataContent(config any) (string, error) {
	nbcJSON, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal nbc, error: %w", err)
	}
	encodedNBCJson := base64.StdEncoding.EncodeToString(nbcJSON)
	customDataYAML := fmt.Sprintf(scriptlessCustomDataTemplate, encodedNBCJson)
	return base64.StdEncoding.EncodeToString([]byte(customDataYAML)), nil
}

func (aKSNodeConfigBuilder *AKSNodeConfigBuilder) validateRequiredFields() error {
	if err := aKSNodeConfigBuilder.validateRequiredStringsNotEmpty(); err != nil {
		return err
	}
	// Add more required fields validations here if needed.
	// For example, check if a required string is of a specific format.
	return nil
}

func (aKSNodeConfigBuilder *AKSNodeConfigBuilder) validateRequiredStringsNotEmpty() error {
	requiredStrings := map[string]string{
		"AuthConfig.SubscriptionId":                     aKSNodeConfigBuilder.aKSNodeConfig.GetAuthConfig().GetSubscriptionId(),
		"ClusterConfig.ResourceGroup":                   aKSNodeConfigBuilder.aKSNodeConfig.GetClusterConfig().GetResourceGroup(),
		"ClusterConfig.Location":                        aKSNodeConfigBuilder.aKSNodeConfig.GetClusterConfig().GetLocation(),
		"ClusterConfig.ClusterNetworkConfig.VnetName":   aKSNodeConfigBuilder.aKSNodeConfig.GetClusterConfig().GetClusterNetworkConfig().GetVnetName(),
		"ClusterConfig.ClusterNetworkConfig.RouteTable": aKSNodeConfigBuilder.aKSNodeConfig.GetClusterConfig().GetClusterNetworkConfig().GetRouteTable(),
		"ApiServerConfig.ApiServerName":                 aKSNodeConfigBuilder.aKSNodeConfig.ApiServerConfig.GetApiServerName(),
	}

	for field, value := range requiredStrings {
		if value == "" {
			return fmt.Errorf("required field %v is missing", field)
		}
	}
	return nil
}

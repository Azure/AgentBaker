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

// NBContractBuilder is a helper struct to build the NBContract (Node Bootstrap Contract).
// It provides methods to apply configuration, get the NBContract object, and validate the contract, etc.
type NBContractBuilder struct {
	// nodeBootstrapConfig is the configuration object for the NBContract (Node Bootstrap Contract).
	nodeBootstrapConfig *Configuration
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

// NewNBContractBuilder creates a new instance of NBContractBuilder and ensures all objects in nodeBootstrapConfig are non-nil.
func NewNBContractBuilder() *NBContractBuilder {
	nbc := Configuration{
		Version: contractVersion,
	}
	ensureConfigsNonNil(&nbc)
	nBCB := &NBContractBuilder{nodeBootstrapConfig: &nbc}
	return nBCB
}

// ApplyConfiguration Applies the configuration to the nodeBootstrapConfig object.
func (nBCB *NBContractBuilder) ApplyConfiguration(config *Configuration) {
	if config == nil {
		return
	}

	// Use deep copy to avoid modifying the original object 'config'.
	if err := nBCB.deepCopy(config, nBCB.nodeBootstrapConfig); err != nil {
		log.Printf("Failed to deep copy the configuration: %v", err)
		ensureConfigsNonNil(nBCB.nodeBootstrapConfig)
	}
}

// GetNodeBootstrapConfig gets the nodeBootstrapConfig object.
func (nBCB *NBContractBuilder) GetNodeBootstrapConfig() *Configuration {
	return nBCB.nodeBootstrapConfig
}

// Deep copy the source object to the destination object.
// Note that the existing value in the destination object will not be cleared
// if the source object doesn't have that field.
func (nBCB *NBContractBuilder) deepCopy(src, dst interface{}) error {
	if src == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	return gob.NewDecoder(&buf).Decode(dst)
}

// ValidateNBContract validates the NBContract.
// It returns an error if the contract is invalid.
// This function should be called after applying all configuration and before sending to downstream component.
func (nBCB *NBContractBuilder) ValidateNBContract() error {
	if err := nBCB.validateRequiredFields(); err != nil {
		return err
	}
	// Add more validations here if needed.

	return nil
}

func (nBCB *NBContractBuilder) GetNodeBootstrapping() (*datamodel.NodeBootstrapping, error) {
	scriptlessCustomData, err := getScriptlessCustomDataContent(nBCB.nodeBootstrapConfig)
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

func (nBCB *NBContractBuilder) validateRequiredFields() error {
	if err := nBCB.validateRequiredStringsNotEmpty(); err != nil {
		return err
	}
	// Add more required fields validations here if needed.
	// For example, check if a required string is of a specific format.
	return nil
}

func (nBCB *NBContractBuilder) validateRequiredStringsNotEmpty() error {
	requiredStrings := map[string]string{
		"AuthConfig.SubscriptionId":                     nBCB.nodeBootstrapConfig.GetAuthConfig().GetSubscriptionId(),
		"ClusterConfig.ResourceGroup":                   nBCB.nodeBootstrapConfig.GetClusterConfig().GetResourceGroup(),
		"ClusterConfig.Location":                        nBCB.nodeBootstrapConfig.GetClusterConfig().GetLocation(),
		"ClusterConfig.ClusterNetworkConfig.VnetName":   nBCB.nodeBootstrapConfig.GetClusterConfig().GetClusterNetworkConfig().GetVnetName(),
		"ClusterConfig.ClusterNetworkConfig.RouteTable": nBCB.nodeBootstrapConfig.GetClusterConfig().GetClusterNetworkConfig().GetRouteTable(),
		"ApiServerConfig.ApiServerName":                 nBCB.nodeBootstrapConfig.ApiServerConfig.GetApiServerName(),
	}

	for field, value := range requiredStrings {
		if value == "" {
			return fmt.Errorf("required field %v is missing", field)
		}
	}
	return nil
}

package nbcontracthelper

import (
	"bytes"
	"encoding/gob"

	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

type NBContractBuilder struct {
	// nodeBootstrapConfig is the configuration object for the NBContract (Node Bootstrap Contract)
	nodeBootstrapConfig *nbcontractv1.Configuration
}

// Check and initialize each field if it is nil
func initializeIfNil[T any](field **T) {
	if *field == nil {
		*field = new(T)
	}
}

// Ensure all objects are non-nil. Please add new objects here.
func ensureConfigsNonNil(nBC *nbcontractv1.Configuration) {
	initializeIfNil(&nBC.KubeBinaryConfig)
	initializeIfNil(&nBC.ApiServerConfig)
	initializeIfNil(&nBC.AuthConfig)
	initializeIfNil(&nBC.ClusterConfig)
	initializeIfNil(&nBC.NetworkConfig)
	initializeIfNil(&nBC.GpuConfig)
	initializeIfNil(&nBC.TlsBootstrappingConfig)
	initializeIfNil(&nBC.KubeletConfig)
	initializeIfNil(&nBC.RuncConfig)
	initializeIfNil(&nBC.ContainerdConfig)
	initializeIfNil(&nBC.TeleportConfig)
	initializeIfNil(&nBC.CustomLinuxOsConfig)
	initializeIfNil(&nBC.HttpProxyConfig)
	initializeIfNil(&nBC.CustomCloudConfig)
	initializeIfNil(&nBC.CustomSearchDomainConfig)
}

// Creates a new instance of NBContractBuilder and ensures all objects in nodeBootstrapConfig are non-nil
func NewNBContractBuilder() *NBContractBuilder {
	nbc := &nbcontractv1.Configuration{}
	ensureConfigsNonNil(nbc)
	nBCB := &NBContractBuilder{nodeBootstrapConfig: nbc}
	return nBCB
}

// Apply the configuration to the nodeBootstrapConfig object
func (nBCB *NBContractBuilder) ApplyConfiguration(config *nbcontractv1.Configuration) {
	if config == nil {
		return
	}

	// Use deep copy to avoid modifying the original object 'config'
	nBCB.deepCopy(config, nBCB.nodeBootstrapConfig)
	ensureConfigsNonNil(nBCB.nodeBootstrapConfig)
}

// Get the nodeBootstrapConfig object
func (nBCB *NBContractBuilder) GetNodeBootstrapConfig() *nbcontractv1.Configuration {
	return nBCB.nodeBootstrapConfig
}

// Deep copy the source object to the destination object.
// Note that the existing value in the destination object will not be cleared
// if the source object doesn't have that field
func (nBCB *NBContractBuilder) deepCopy(src, dst interface{}) error {
	if src == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	if err := gob.NewDecoder(&buf).Decode(dst); err != nil {
		return err
	}
	return nil
}

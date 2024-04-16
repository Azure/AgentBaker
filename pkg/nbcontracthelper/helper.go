package nbcontracthelper

import (
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

type NBContractBuilder struct {
	// NBContractConfiguration is the configuration object for the NBContract (Node Bootstrap Contract)
	nBContractConfiguration *nbcontractv1.Configuration
}

// Check and initialize each field if it is nil
func initializeIfNil[T any](field **T) {
	if *field == nil {
		*field = new(T)
	}
}

// Ensure all objects are non-nil. Please add new objects here.
func (nBCB *NBContractBuilder) ensureConfigsNonNil() {
	initializeIfNil(&nBCB.nBContractConfiguration.KubeBinaryConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.ApiServerConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.AuthConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.ClusterConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.NetworkConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.GpuConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.TlsBootstrappingConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.KubeletConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.RuncConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.ContainerdConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.TeleportConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.CustomLinuxOsConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.HttpProxyConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.CustomCloudConfig)
	initializeIfNil(&nBCB.nBContractConfiguration.CustomSearchDomainConfig)
}

// Creates a new instance of NBContractConfig and ensures all objects are non-nil
func NewNBContractBuilder() *NBContractBuilder {
	nBCB := &NBContractBuilder{
		nBContractConfiguration: &nbcontractv1.Configuration{
			KubeBinaryConfig:         &nbcontractv1.KubeBinaryConfig{},
			ApiServerConfig:          &nbcontractv1.ApiServerConfig{},
			AuthConfig:               &nbcontractv1.AuthConfig{},
			ClusterConfig:            &nbcontractv1.ClusterConfig{},
			NetworkConfig:            &nbcontractv1.NetworkConfig{},
			GpuConfig:                &nbcontractv1.GPUConfig{},
			TlsBootstrappingConfig:   &nbcontractv1.TLSBootstrappingConfig{},
			KubeletConfig:            &nbcontractv1.KubeletConfig{},
			RuncConfig:               &nbcontractv1.RuncConfig{},
			ContainerdConfig:         &nbcontractv1.ContainerdConfig{},
			TeleportConfig:           &nbcontractv1.TeleportConfig{},
			CustomLinuxOsConfig:      &nbcontractv1.CustomLinuxOSConfig{},
			HttpProxyConfig:          &nbcontractv1.HTTPProxyConfig{},
			CustomCloudConfig:        &nbcontractv1.CustomCloudConfig{},
			CustomSearchDomainConfig: &nbcontractv1.CustomSearchDomainConfig{},
		},
	}
	return nBCB
}

// Apply the configuration to the NBContractConfig nbContractConfiguration object
func (nBCB *NBContractBuilder) ApplyConfiguration(config *nbcontractv1.Configuration) {
	if config == nil {
		return
	}
	nBCB.nBContractConfiguration = config
	nBCB.ensureConfigsNonNil()
}

// Get the NBContractConfiguration object
func (nBCB *NBContractBuilder) GetNBContractConfiguration() *nbcontractv1.Configuration {
	return nBCB.nBContractConfiguration
}

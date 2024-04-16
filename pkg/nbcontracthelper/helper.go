package nbcontracthelper

import (
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

type NBContractConfig struct {
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
func (nbcc *NBContractConfig) ensureConfigsNonNil() {
	initializeIfNil(&nbcc.nBContractConfiguration.KubeBinaryConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.ApiServerConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.AuthConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.ClusterConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.NetworkConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.GpuConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.TlsBootstrappingConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.KubeletConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.RuncConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.ContainerdConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.TeleportConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.CustomLinuxOsConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.HttpProxyConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.CustomCloudConfig)
	initializeIfNil(&nbcc.nBContractConfiguration.CustomSearchDomainConfig)
}

// Creates a new instance of NBContractConfig and ensures all objects are non-nil
func NewNBContractConfiguration() *NBContractConfig {
	nbcc := &NBContractConfig{
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
	return nbcc
}

// Apply the configuration to the NBContractConfig nbContractConfiguration object
func (nbcc *NBContractConfig) ApplyConfiguration(config *nbcontractv1.Configuration) {
	if config == nil {
		return
	}
	nbcc.nBContractConfiguration = config
	nbcc.ensureConfigsNonNil()
}

// Get the NBContractConfiguration object
func (nbcc *NBContractConfig) GetNBContractConfiguration() *nbcontractv1.Configuration {
	return nbcc.nBContractConfiguration
}

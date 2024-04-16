package nbcontracthelper

import (
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

// Check and initialize each field if it is nil
func initializeIfNil[T any](field **T) {
	if *field == nil {
		*field = new(T)
	}
}

// EnsureConfigsNonNil checks if the config is nil and initializes it if it is.
func ensureConfigsNonNil(v *nbcontractv1.Configuration) *nbcontractv1.Configuration {
	if v == nil {
		v = &nbcontractv1.Configuration{}
	}

	initializeIfNil(&v.KubeBinaryConfig)
	initializeIfNil(&v.ApiServerConfig)
	initializeIfNil(&v.AuthConfig)
	initializeIfNil(&v.ClusterConfig)
	initializeIfNil(&v.NetworkConfig)
	initializeIfNil(&v.GpuConfig)
	initializeIfNil(&v.TlsBootstrappingConfig)
	initializeIfNil(&v.KubeletConfig)
	initializeIfNil(&v.RuncConfig)
	initializeIfNil(&v.ContainerdConfig)
	initializeIfNil(&v.TeleportConfig)
	initializeIfNil(&v.CustomLinuxOsConfig)
	initializeIfNil(&v.HttpProxyConfig)
	initializeIfNil(&v.CustomCloudConfig)
	initializeIfNil(&v.CustomSearchDomainConfig)

	return v
}

// Creates a new instance of nbcontractv1.Configuration and ensures that
// all fields are non-nil unless it's set as proto3 explicit presence (with label optional).
func NewNBContractConfiguration(v *nbcontractv1.Configuration) *nbcontractv1.Configuration {
	return ensureConfigsNonNil(v)
}

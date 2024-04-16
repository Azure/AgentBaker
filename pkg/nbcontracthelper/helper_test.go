package nbcontracthelper

import (
	"reflect"
	"testing"

	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

func TestNewNBContractConfiguration(t *testing.T) {
	wantedResult := &nbcontractv1.Configuration{
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
	}
	tests := []struct {
		name string
		want *nbcontractv1.Configuration
	}{
		{
			name: "Test with nil configuration",
			want: wantedResult,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewNBContractConfiguration().nBContractConfiguration; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewNBContractConfiguration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNBContractConfig_ApplyConfiguration(t *testing.T) {
	type fields struct {
		nBContractConfiguration *nbcontractv1.Configuration
	}
	wantedResult := &nbcontractv1.Configuration{
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
	}
	tests := []struct {
		name   string
		fields fields
		want   *nbcontractv1.Configuration
	}{
		{
			name: "Test with nil configuration",
			fields: fields{
				nBContractConfiguration: &nbcontractv1.Configuration{},
			},
			want: wantedResult,
		},
		{
			name: "Apply nil AuthConfig configuration and expect AuthConfig in nBContractConfiguration to be non-nil",
			fields: fields{
				nBContractConfiguration: &nbcontractv1.Configuration{
					AuthConfig: nil,
				},
			},
			want: wantedResult,
		},
		{
			name: "Apply some configurations and expect them to be applied",
			fields: fields{
				nBContractConfiguration: &nbcontractv1.Configuration{
					AuthConfig: &nbcontractv1.AuthConfig{
						TargetCloud: "some-cloud",
					},
					LinuxAdminUsername: "testuser",
				},
			},
			want: func() *nbcontractv1.Configuration {
				tmpResult := NewNBContractConfiguration().nBContractConfiguration
				tmpResult.AuthConfig.TargetCloud = "some-cloud"
				tmpResult.LinuxAdminUsername = "testuser"
				return tmpResult
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nbcc := NewNBContractConfiguration()
			nbcc.ApplyConfiguration(tt.fields.nBContractConfiguration)
			if got := nbcc.nBContractConfiguration; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ApplyConfiguration() = %v, want %v", got, tt.want)
			}
		})
	}
}

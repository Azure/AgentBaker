package parser

import (
	"reflect"
	"testing"

	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

func Test_ensureConfigsNonNil(t *testing.T) {
	type args struct {
		v *nbcontractv1.Configuration
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
		name string
		args args
		want *nbcontractv1.Configuration
	}{
		{
			name: "Test with nil configuration",
			args: args{
				v: nil,
			},
			want: wantedResult,
		},
		{
			name: "Test with non-nil configuration and all nil Configs",
			args: args{
				v: &nbcontractv1.Configuration{},
			},
			want: wantedResult,
		},
		{
			name: "Test with non-nil configuration and partially nil Configs",
			args: args{
				v: &nbcontractv1.Configuration{
					KubeBinaryConfig: &nbcontractv1.KubeBinaryConfig{},
				},
			},
			want: wantedResult,
		},
		{
			name: "Test with non-nil configuration and all non-nil Configs",
			args: args{
				v: &nbcontractv1.Configuration{
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
			},
			want: wantedResult,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ensureConfigsNonNil(tt.args.v); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ensureConfigsNonNil() = %v, want %v", got, tt.want)
			}
		})
	}
}

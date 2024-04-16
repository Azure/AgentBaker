package nbcontracthelper

import (
	"log"
	"reflect"
	"testing"

	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
	"github.com/Azure/go-autorest/autorest/to"
)

func TestNewNBContractBuilder(t *testing.T) {
	wantedResult := &nbcontractv1.Configuration{
		KubeBinaryConfig: &nbcontractv1.KubeBinaryConfig{},
		ApiServerConfig:  &nbcontractv1.ApiServerConfig{},
		AuthConfig:       &nbcontractv1.AuthConfig{},
		ClusterConfig: &nbcontractv1.ClusterConfig{
			LoadBalancerConfig:   &nbcontractv1.LoadBalancerConfig{},
			ClusterNetworkConfig: &nbcontractv1.ClusterNetworkConfig{},
		},
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
			if got := NewNBContractBuilder().nodeBootstrapConfig; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewNBContractConfiguration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNBContractBuilder_ApplyConfiguration(t *testing.T) {
	type fields struct {
		nBContractConfiguration *nbcontractv1.Configuration
	}
	wantedResult := &nbcontractv1.Configuration{
		KubeBinaryConfig: &nbcontractv1.KubeBinaryConfig{},
		ApiServerConfig:  &nbcontractv1.ApiServerConfig{},
		AuthConfig:       &nbcontractv1.AuthConfig{},
		ClusterConfig: &nbcontractv1.ClusterConfig{
			LoadBalancerConfig:   &nbcontractv1.LoadBalancerConfig{},
			ClusterNetworkConfig: &nbcontractv1.ClusterNetworkConfig{},
		},
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
				tmpResult := NewNBContractBuilder().nodeBootstrapConfig
				tmpResult.AuthConfig.TargetCloud = "some-cloud"
				tmpResult.LinuxAdminUsername = "testuser"
				return tmpResult
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewNBContractBuilder()
			builder.ApplyConfiguration(tt.fields.nBContractConfiguration)
			if got := builder.nodeBootstrapConfig; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ApplyConfiguration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNBContractBuilder_deepCopy(t *testing.T) {
	type Teststruct struct {
		A string
		B *int
		C bool
	}
	type args struct {
		src *Teststruct
		dst *Teststruct
	}
	tests := []struct {
		name        string
		args        args
		want        *Teststruct
		isChangeDst bool
	}{
		{
			name: "Test with empty src",
			args: args{
				src: &Teststruct{},
				dst: &Teststruct{},
			},
			want:        &Teststruct{},
			isChangeDst: false,
		},
		{
			name: "Test with non-empty src",
			args: args{
				src: &Teststruct{
					A: "some-string",
					B: to.IntPtr(123),
					C: true,
				},
				dst: &Teststruct{},
			},
			want: &Teststruct{
				A: "some-string",
				B: to.IntPtr(123),
				C: true,
			},
			isChangeDst: false,
		},
		{
			name: "Test with dst which has some existing values. Expect them to be overwritten",
			args: args{
				src: &Teststruct{
					A: "some-string",
					B: to.IntPtr(123),
				},
				dst: &Teststruct{
					A: "some-other-string",
					B: to.IntPtr(456),
					C: false,
				},
			},
			want: &Teststruct{
				A: "some-string",
				B: to.IntPtr(123),
				C: false,
			},
			isChangeDst: false,
		},
		{
			name: "After deepCopy, changes in dst should not affect src",
			args: args{
				src: &Teststruct{
					A: "some-string",
					B: to.IntPtr(123),
				},
				dst: &Teststruct{},
			},
			want: &Teststruct{
				A: "some-string",
				B: to.IntPtr(123),
			},
			isChangeDst: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nBCB := &NBContractBuilder{
				nodeBootstrapConfig: &nbcontractv1.Configuration{},
			}
			nBCB.deepCopy(tt.args.src, tt.args.dst)
			log.Printf("dst = %v, src %v", tt.args.dst, tt.args.src)
			if got := tt.args.dst; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
			if tt.isChangeDst {
				tt.args.dst.A = "some-other-string"
				tt.args.dst.B = to.IntPtr(456)
				if got := tt.args.src; !reflect.DeepEqual(got, tt.want) {
					t.Errorf("src = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

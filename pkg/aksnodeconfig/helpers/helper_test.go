package helpers

import (
	"log"
	"reflect"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/pkg/aksnodeconfig/v1"
	"github.com/Azure/go-autorest/autorest/to"
)

func TestNewAKSNodeConfigBuilder(t *testing.T) {
	wantedResult := aksnodeconfigv1.Configuration{
		Version:          contractVersion,
		KubeBinaryConfig: &aksnodeconfigv1.KubeBinaryConfig{},
		ApiServerConfig:  &aksnodeconfigv1.ApiServerConfig{},
		AuthConfig:       &aksnodeconfigv1.AuthConfig{},
		ClusterConfig: &aksnodeconfigv1.ClusterConfig{
			LoadBalancerConfig:   &aksnodeconfigv1.LoadBalancerConfig{},
			ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{},
		},
		GpuConfig:              &aksnodeconfigv1.GPUConfig{},
		NetworkConfig:          &aksnodeconfigv1.NetworkConfig{},
		TlsBootstrappingConfig: &aksnodeconfigv1.TLSBootstrappingConfig{},
		KubeletConfig:          &aksnodeconfigv1.KubeletConfig{},
		RuncConfig:             &aksnodeconfigv1.RuncConfig{},
		ContainerdConfig:       &aksnodeconfigv1.ContainerdConfig{},
		TeleportConfig:         &aksnodeconfigv1.TeleportConfig{},
		CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOSConfig{
			SysctlConfig: &aksnodeconfigv1.SysctlConfig{},
			UlimitConfig: &aksnodeconfigv1.UlimitConfig{},
		},
		HttpProxyConfig:          &aksnodeconfigv1.HTTPProxyConfig{},
		CustomCloudConfig:        &aksnodeconfigv1.CustomCloudConfig{},
		CustomSearchDomainConfig: &aksnodeconfigv1.CustomSearchDomainConfig{},
	}
	tests := []struct {
		name string
		want *aksnodeconfigv1.Configuration
	}{
		{
			name: "Test with nil configuration",
			want: &wantedResult,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewAKSNodeConfigBuilder().aksNodeConfig; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAKSNodeConfigConfiguration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAKSNodeConfigBuilder_ApplyConfiguration(t *testing.T) {
	type fields struct {
		AKSNodeConfigConfiguration *aksnodeconfigv1.Configuration
	}
	wantedResult := &aksnodeconfigv1.Configuration{
		Version:          contractVersion,
		KubeBinaryConfig: &aksnodeconfigv1.KubeBinaryConfig{},
		ApiServerConfig:  &aksnodeconfigv1.ApiServerConfig{},
		AuthConfig:       &aksnodeconfigv1.AuthConfig{},
		ClusterConfig: &aksnodeconfigv1.ClusterConfig{
			LoadBalancerConfig:   &aksnodeconfigv1.LoadBalancerConfig{},
			ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{},
		},
		GpuConfig:              &aksnodeconfigv1.GPUConfig{},
		NetworkConfig:          &aksnodeconfigv1.NetworkConfig{},
		TlsBootstrappingConfig: &aksnodeconfigv1.TLSBootstrappingConfig{},
		KubeletConfig:          &aksnodeconfigv1.KubeletConfig{},
		RuncConfig:             &aksnodeconfigv1.RuncConfig{},
		ContainerdConfig:       &aksnodeconfigv1.ContainerdConfig{},
		TeleportConfig:         &aksnodeconfigv1.TeleportConfig{},
		CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOSConfig{
			SysctlConfig: &aksnodeconfigv1.SysctlConfig{},
			UlimitConfig: &aksnodeconfigv1.UlimitConfig{},
		},
		HttpProxyConfig:          &aksnodeconfigv1.HTTPProxyConfig{},
		CustomCloudConfig:        &aksnodeconfigv1.CustomCloudConfig{},
		CustomSearchDomainConfig: &aksnodeconfigv1.CustomSearchDomainConfig{},
	}
	tests := []struct {
		name   string
		fields fields
		want   *aksnodeconfigv1.Configuration
	}{
		{
			name: "Test with nil configuration",
			fields: fields{
				AKSNodeConfigConfiguration: &aksnodeconfigv1.Configuration{},
			},
			want: wantedResult,
		},
		{
			name: "Apply nil AuthConfig configuration and expect AuthConfig in AKSNodeConfigConfiguration to be non-nil",
			fields: fields{
				AKSNodeConfigConfiguration: &aksnodeconfigv1.Configuration{
					AuthConfig: nil,
				},
			},
			want: wantedResult,
		},
		{
			name: "Apply some configurations and expect them to be applied",
			fields: fields{
				AKSNodeConfigConfiguration: &aksnodeconfigv1.Configuration{
					CustomCloudConfig: &aksnodeconfigv1.CustomCloudConfig{
						CustomCloudEnvName: "some-cloud",
					},
					LinuxAdminUsername: "testuser",
				},
			},
			want: func() *aksnodeconfigv1.Configuration {
				tmpResult := NewAKSNodeConfigBuilder().aksNodeConfig
				tmpResult.CustomCloudConfig.CustomCloudEnvName = "some-cloud"
				tmpResult.LinuxAdminUsername = "testuser"
				return tmpResult
			}(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewAKSNodeConfigBuilder()
			builder.ApplyConfiguration(tt.fields.AKSNodeConfigConfiguration)
			if got := builder.aksNodeConfig; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ApplyConfiguration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeepCopy(t *testing.T) {
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
			if err := deepCopy(tt.args.src, tt.args.dst); err != nil {
				log.Printf("Failed to deep copy the configuration: %v", err)
			}
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

func TestAKSNodeConfigBuilder_validateRequiredFields(t *testing.T) {
	type fields struct {
		nodeBootstrapConfig *aksnodeconfigv1.Configuration
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Test with nil AuthConfig and expect error",
			fields: fields{
				nodeBootstrapConfig: &aksnodeconfigv1.Configuration{
					AuthConfig: nil,
				},
			},
			wantErr: true,
		},
		{
			name: "Test with empty SubscriptionId and expect error",
			fields: fields{
				nodeBootstrapConfig: &aksnodeconfigv1.Configuration{
					AuthConfig: &aksnodeconfigv1.AuthConfig{
						SubscriptionId: "",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Test with required fields and expect no error",
			fields: fields{
				nodeBootstrapConfig: &aksnodeconfigv1.Configuration{
					AuthConfig: &aksnodeconfigv1.AuthConfig{
						SubscriptionId: "some-subscription-id",
						TenantId:       "some-tenant-id",
					},
					ClusterConfig: &aksnodeconfigv1.ClusterConfig{
						ResourceGroup: "some-resource-group",
						Location:      "some-location",
						ClusterNetworkConfig: &aksnodeconfigv1.ClusterNetworkConfig{
							RouteTable: "some-route-table",
							VnetName:   "some-vnet-name",
						},
					},
					CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOSConfig{
						SwapFileSize: 2048,
					},
					ApiServerConfig: &aksnodeconfigv1.ApiServerConfig{
						ApiServerName: "some-api-server-name",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aksNodeConfigBuilder := NewAKSNodeConfigBuilder()
			aksNodeConfigBuilder.ApplyConfiguration(tt.fields.nodeBootstrapConfig)
			err := aksNodeConfigBuilder.validateRequiredFields()
			if (err != nil) != tt.wantErr {
				t.Errorf("aksNodeConfigBuilder.validateRequiredFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

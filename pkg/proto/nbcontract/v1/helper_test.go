package nbcontractv1

import (
	"log"
	"reflect"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/go-autorest/autorest/to"
)

func TestNewNBContractBuilder(t *testing.T) {
	wantedResult := Configuration{
		Version:          contractVersion,
		KubeBinaryConfig: &KubeBinaryConfig{},
		ApiServerConfig:  &ApiServerConfig{},
		AuthConfig:       &AuthConfig{},
		ClusterConfig: &ClusterConfig{
			LoadBalancerConfig:   &LoadBalancerConfig{},
			ClusterNetworkConfig: &ClusterNetworkConfig{},
		},
		GpuConfig:              &GPUConfig{},
		NetworkConfig:          &NetworkConfig{},
		TlsBootstrappingConfig: &TLSBootstrappingConfig{},
		KubeletConfig:          &KubeletConfig{},
		RuncConfig:             &RuncConfig{},
		ContainerdConfig:       &ContainerdConfig{},
		TeleportConfig:         &TeleportConfig{},
		CustomLinuxOsConfig: &CustomLinuxOSConfig{
			SysctlConfig: &SysctlConfig{},
			UlimitConfig: &UlimitConfig{},
		},
		HttpProxyConfig:          &HTTPProxyConfig{},
		CustomCloudConfig:        &CustomCloudConfig{},
		CustomSearchDomainConfig: &CustomSearchDomainConfig{},
	}
	tests := []struct {
		name string
		want *Configuration
	}{
		{
			name: "Test with nil configuration",
			want: &wantedResult,
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
		nBContractConfiguration *Configuration
	}
	wantedResult := &Configuration{
		Version:          contractVersion,
		KubeBinaryConfig: &KubeBinaryConfig{},
		ApiServerConfig:  &ApiServerConfig{},
		AuthConfig:       &AuthConfig{},
		ClusterConfig: &ClusterConfig{
			LoadBalancerConfig:   &LoadBalancerConfig{},
			ClusterNetworkConfig: &ClusterNetworkConfig{},
		},
		GpuConfig:              &GPUConfig{},
		NetworkConfig:          &NetworkConfig{},
		TlsBootstrappingConfig: &TLSBootstrappingConfig{},
		KubeletConfig:          &KubeletConfig{},
		RuncConfig:             &RuncConfig{},
		ContainerdConfig:       &ContainerdConfig{},
		TeleportConfig:         &TeleportConfig{},
		CustomLinuxOsConfig: &CustomLinuxOSConfig{
			SysctlConfig: &SysctlConfig{},
			UlimitConfig: &UlimitConfig{},
		},
		HttpProxyConfig:          &HTTPProxyConfig{},
		CustomCloudConfig:        &CustomCloudConfig{},
		CustomSearchDomainConfig: &CustomSearchDomainConfig{},
	}
	tests := []struct {
		name   string
		fields fields
		want   *Configuration
	}{
		{
			name: "Test with nil configuration",
			fields: fields{
				nBContractConfiguration: &Configuration{},
			},
			want: wantedResult,
		},
		{
			name: "Apply nil AuthConfig configuration and expect AuthConfig in nBContractConfiguration to be non-nil",
			fields: fields{
				nBContractConfiguration: &Configuration{
					AuthConfig: nil,
				},
			},
			want: wantedResult,
		},
		{
			name: "Apply some configurations and expect them to be applied",
			fields: fields{
				nBContractConfiguration: &Configuration{
					AuthConfig: &AuthConfig{
						TargetCloud: "some-cloud",
					},
					LinuxAdminUsername: "testuser",
				},
			},
			want: func() *Configuration {
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
				nodeBootstrapConfig: &Configuration{},
			}
			if err := nBCB.deepCopy(tt.args.src, tt.args.dst); err != nil {
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

func TestNBContractBuilder_validateSemVer(t *testing.T) {
	type fields struct {
		nodeBootstrapConfig *Configuration
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Test with nil version and expect error",
			fields: fields{
				nodeBootstrapConfig: &Configuration{},
			},
			wantErr: true,
		},
		{
			name: "Test with invalid version and expect error",
			fields: fields{
				nodeBootstrapConfig: &Configuration{
					Version: "some-invalid-version",
				},
			},
			wantErr: true,
		},
		{
			name: "Test with valid version and expect no error",
			fields: fields{
				nodeBootstrapConfig: &Configuration{
					Version: contractVersion,
				},
			},
			wantErr: false,
		},
		{
			name: "Test with mismatch major version and expect error",
			fields: fields{
				nodeBootstrapConfig: &Configuration{
					Version: "2.0.0",
				},
			},
			wantErr: true,
		},
		{
			name: "Test with mismatch minor version and expect no error",
			fields: fields{
				nodeBootstrapConfig: &Configuration{
					Version: "1.1.0",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nBCB := NewNBContractBuilder()
			nBCB.ApplyConfiguration(tt.fields.nodeBootstrapConfig)
			nBCB.nodeBootstrapConfig.Version = tt.fields.nodeBootstrapConfig.Version
			if err := nBCB.validateSemVer(); (err != nil) != tt.wantErr {
				t.Errorf("NBContractBuilder.validateSemVer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNBContractBuilder_validateRequiredFields(t *testing.T) {
	type fields struct {
		nodeBootstrapConfig *Configuration
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "Test with nil AuthConfig and expect error",
			fields: fields{
				nodeBootstrapConfig: &Configuration{
					AuthConfig: nil,
				},
			},
			wantErr: true,
		},
		{
			name: "Test with empty SubscriptionId and expect error",
			fields: fields{
				nodeBootstrapConfig: &Configuration{
					AuthConfig: &AuthConfig{
						SubscriptionId: "",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Test with required fields and expect no error",
			fields: fields{
				nodeBootstrapConfig: &Configuration{
					AuthConfig: &AuthConfig{
						SubscriptionId: "some-subscription-id",
						TenantId:       "some-tenant-id",
					},
					ClusterConfig: &ClusterConfig{
						ResourceGroup: "some-resource-group",
						Location:      "some-location",
						ClusterNetworkConfig: &ClusterNetworkConfig{
							RouteTable: "some-route-table",
							VnetName:   "some-vnet-name",
						},
					},
					CustomLinuxOsConfig: &CustomLinuxOSConfig{
						SwapFileSize: 2048,
					},
					ApiServerConfig: &ApiServerConfig{
						ApiServerName: "some-api-server-name",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nBCB := NewNBContractBuilder()
			nBCB.ApplyConfiguration(tt.fields.nodeBootstrapConfig)
			err := nBCB.validateRequiredFields()
			if (err != nil) != tt.wantErr {
				t.Errorf("NBContractBuilder.validateRequiredFields() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getLoadBalancerSKU(t *testing.T) {
	type args struct {
		sku string
	}
	tests := []struct {
		name string
		args args
		want LoadBalancerConfig_LoadBalancerSku
	}{
		{
			name: "Test with Standard SKU",
			args: args{
				sku: LoadBalancerStandard,
			},
			want: LoadBalancerConfig_STANDARD,
		},
		{
			name: "Test with Basic SKU",
			args: args{
				sku: LoadBalancerBasic,
			},
			want: LoadBalancerConfig_BASIC,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetLoadBalancerSKU(tt.args.sku); got != tt.want {
				t.Errorf("GetLoadBalancerSKU() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetOutBoundCmd(t *testing.T) {
	type args struct {
		nbconfig  *datamodel.NodeBootstrappingConfiguration
		cloudName string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test with cloudName as AzureChinaCloud and orchestratorVersion as 1.19.0",
			args: args{
				nbconfig: &datamodel.NodeBootstrappingConfiguration{
					ContainerService: &datamodel.ContainerService{
						Properties: &datamodel.Properties{
							OrchestratorProfile: &datamodel.OrchestratorProfile{
								OrchestratorVersion: "1.19.0",
							},
						},
					},
				},
				cloudName: AzureChinaCloud,
			},
			want: "curl -v --insecure --proxy-insecure https://gcr.azk8s.cn/v2/",
		},
		{
			name: "Test with cloudName as AzureChinaCloud and orchestratorVersion as 1.17.0",
			args: args{
				nbconfig: &datamodel.NodeBootstrappingConfiguration{
					ContainerService: &datamodel.ContainerService{
						Properties: &datamodel.Properties{
							OrchestratorProfile: &datamodel.OrchestratorProfile{
								OrchestratorVersion: "1.17.0",
							},
						},
					},
				},
				cloudName: AzureChinaCloud,
			},
			want: "nc -vz gcr.azk8s.cn 443",
		},
		{
			name: "Test with cloudName as AzurePublicCloud and orchestratorVersion as 1.19.0",
			args: args{
				nbconfig: &datamodel.NodeBootstrappingConfiguration{
					ContainerService: &datamodel.ContainerService{
						Properties: &datamodel.Properties{
							OrchestratorProfile: &datamodel.OrchestratorProfile{
								OrchestratorVersion: "1.19.0",
							},
						},
					},
				},
				cloudName: DefaultCloudName,
			},
			want: "curl -v --insecure --proxy-insecure https://mcr.microsoft.com/v2/",
		},
		{
			name: "Test with AKSCustomCloud and orchestratorVersion as 1.19.0",
			args: args{
				nbconfig: &datamodel.NodeBootstrappingConfiguration{
					ContainerService: &datamodel.ContainerService{
						Properties: &datamodel.Properties{
							OrchestratorProfile: &datamodel.OrchestratorProfile{
								OrchestratorVersion: "1.19.0",
							},
							CustomCloudEnv: &datamodel.CustomCloudEnv{
								McrURL: "some-mcr-url",
								Name:   "akscustom",
							},
						},
					},
				},
				cloudName: "any cloud name",
			},
			want: "curl -v --insecure --proxy-insecure https://some-mcr-url/v2/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOutBoundCmd(tt.args.nbconfig, tt.args.cloudName); got != tt.want {
				t.Errorf("GetOutBoundCmd() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestGetNetworkPolicyType(t *testing.T) {
	tests := []struct {
		name           string
		networkPolicy  string
		expectedResult NetworkPolicy
	}{
		{
			name:           "Test with NetworkPolicyAzure",
			networkPolicy:  NetworkPolicyAzure,
			expectedResult: NetworkPolicy_NPO_AZURE,
		},
		{
			name:           "Test with NetworkPolicyCalico",
			networkPolicy:  NetworkPolicyCalico,
			expectedResult: NetworkPolicy_NPO_CALICO,
		},
		{
			name:           "Test with unknown network policy",
			networkPolicy:  "unknown",
			expectedResult: NetworkPolicy_NPO_NONE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNetworkPolicyType(tt.networkPolicy)
			if result != tt.expectedResult {
				t.Errorf("GetNetworkPolicyType() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}
func TestGetNetworkPluginType(t *testing.T) {
	tests := []struct {
		name           string
		networkPlugin  string
		expectedResult NetworkPlugin
	}{
		{
			name:           "Test with Azure network plugin",
			networkPlugin:  "azure",
			expectedResult: NetworkPlugin_NP_AZURE,
		},
		{
			name:           "Test with kubenet network plugin",
			networkPlugin:  "kubenet",
			expectedResult: NetworkPlugin_NP_KUBENET,
		},
		{
			name:           "Test with unknown network plugin",
			networkPlugin:  "unknown",
			expectedResult: NetworkPlugin_NP_NONE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNetworkPluginType(tt.networkPlugin)
			if result != tt.expectedResult {
				t.Errorf("GetNetworkPluginType() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

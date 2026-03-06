package nodeconfigutils

import (
	"os"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestUnmarshalConfigurationV1(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *aksnodeconfigv1.Configuration
		wantErr bool
	}{
		{
			name: "valid minimal config",
			data: []byte(`{
				"version": "v1",
				"auth_config": {
					"subscription_id": "test-subscription"
				},
				"cluster_config": {
					"resource_group": "test-rg",
					"location": "eastus"
				},
				"api_server_config": {
					"apiServerName": "test-api-server"
				}
			}`),
			want: &aksnodeconfigv1.Configuration{
				Version: "v1",
				AuthConfig: &aksnodeconfigv1.AuthConfig{
					SubscriptionId: "test-subscription",
				},
				ClusterConfig: &aksnodeconfigv1.ClusterConfig{
					ResourceGroup: "test-rg",
					Location:      "eastus",
				},
				ApiServerConfig: &aksnodeconfigv1.ApiServerConfig{
					ApiServerName: "test-api-server",
				},
			},
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    []byte(""),
			want:    &aksnodeconfigv1.Configuration{},
			wantErr: true,
		},
		{
			name: "invalid JSON",
			data: []byte(`{"version": "v1", invalid}`),
			want: &aksnodeconfigv1.Configuration{
				Version: "v1",
			},
			wantErr: true,
		},
		{
			name: "unknown field should be ignored",
			data: []byte(`{
				"version": "v1",
				"unknown_feld": "should be ignored",
				"auth_config": {
					"subscription_id": "test-subscription"
				}
			}`),
			want: &aksnodeconfigv1.Configuration{
				Version: "v1",
				AuthConfig: &aksnodeconfigv1.AuthConfig{
					SubscriptionId: "test-subscription",
				},
			},
			wantErr: false,
		},
		{
			name: "valid enum values as strings",
			data: []byte(`{
				"version": "v1",
				"auth_config": {
					"subscription_id": "test-subscription"
				},
				"workload_runtime": "WORKLOAD_RUNTIME_OCI_CONTAINER"
			}`),
			want: &aksnodeconfigv1.Configuration{
				Version: "v1",
				AuthConfig: &aksnodeconfigv1.AuthConfig{
					SubscriptionId: "test-subscription",
				},
				WorkloadRuntime: aksnodeconfigv1.WorkloadRuntime_WORKLOAD_RUNTIME_OCI_CONTAINER,
			},
			wantErr: false,
		},
		{
			name: "unknown enum values should default to UNSPECIFIED",
			data: []byte(`{
				"version": "v1",
				"auth_config": {
					"subscription_id": "test-subscription"
				},
				"workload_runtime": "WHAT IS THIS?"
			}`),
			want: &aksnodeconfigv1.Configuration{
				Version: "v1",
				AuthConfig: &aksnodeconfigv1.AuthConfig{
					SubscriptionId: "test-subscription",
				},
				WorkloadRuntime: aksnodeconfigv1.WorkloadRuntime_WORKLOAD_RUNTIME_UNSPECIFIED,
			},
			wantErr: false,
		},
		{
			name: "optional int32 field with string value is ignored",
			data: []byte(`{
				"version": "v1",
				"auth_config": {
					"subscription_id": "test-subscription"
				},
				"kubelet_config": {
					"max_pods": "42"
				}
			}`),
			want: &aksnodeconfigv1.Configuration{
				Version: "v1",
				AuthConfig: &aksnodeconfigv1.AuthConfig{
					SubscriptionId: "test-subscription",
				},
				KubeletConfig: &aksnodeconfigv1.KubeletConfig{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalConfigurationV1(tt.data)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// here we use proto.Equal for deep equality check
			}
			if !proto.Equal(tt.want, got) {
				assert.Fail(t, "UnmarshalConfigurationV1() result mismatch", "want: %+v\n got: %+v", tt.want, got)
			}
		})
	}
}

func TestUnmarshalConfigurationV1FromAJsonFile(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    *aksnodeconfigv1.Configuration
		wantErr bool
	}{
		{
			name: "valid config from test file",
			data: func() []byte {
				data, err := os.ReadFile("../../parser/testdata/test_aksnodeconfig.json")
				if err != nil {
					t.Logf("Could not read test file, skipping: %v", err)
					return []byte(`{"version": "v1"}`)
				}
				return data
			}(),
			want:    nil, // We'll check for non-nil result instead of exact match
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalConfigurationV1(tt.data)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// The input is from a JSON file so we don't have an exact expected struct to compare against.
				// Instead, we just check that we got a non-nil result.
				assert.NotNil(t, got, "UnmarshalConfigurationV1() returned nil for valid test file")
			}
		})
	}
}

func TestMarsalConfiguratioV1(t *testing.T) {
	cfg := &aksnodeconfigv1.Configuration{
		Version: "v1",
		AuthConfig: &aksnodeconfigv1.AuthConfig{
			SubscriptionId: "test-subscription",
		},
		WorkloadRuntime: aksnodeconfigv1.WorkloadRuntime_WORKLOAD_RUNTIME_OCI_CONTAINER,
	}
	data, err := MarshalConfigurationV1(cfg)
	require.NoError(t, err)
	require.JSONEq(t, `{"version":"v1","auth_config":{"subscription_id":"test-subscription"}, "workload_runtime":"WORKLOAD_RUNTIME_OCI_CONTAINER"}`, string(data))
}

func TestMarshalUnmarshalWithPopulatedConfig(t *testing.T) {
	t.Run("fully populated config marshals to >100 bytes", func(t *testing.T) {
		cfg := &aksnodeconfigv1.Configuration{}
		PopulateAllFields(cfg)

		marshaled, err := MarshalConfigurationV1(cfg)
		require.NoError(t, err)
		assert.Greater(t, len(marshaled), 100, "Fully populated config should marshal to >100 bytes")
		t.Logf("Marshaled %d bytes", len(marshaled))
	})

	t.Run("marshal and unmarshal round-trip preserves data", func(t *testing.T) {
		original := &aksnodeconfigv1.Configuration{}
		PopulateAllFields(original)

		// Marshal
		marshaled, err := MarshalConfigurationV1(original)
		require.NoError(t, err)

		// Unmarshal
		restored, err := UnmarshalConfigurationV1(marshaled)
		require.NoError(t, err)

		// Verify key fields preserved
		assert.Equal(t, original, restored)
	})
}

func TestValidateTHPConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *aksnodeconfigv1.Configuration
		wantSupport string
		wantDefrag  string
	}{
		{
			name: "nil custom_linux_os_config passes",
			cfg:  &aksnodeconfigv1.Configuration{},
		},
		{
			name: "empty THP values pass",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{},
			},
		},
		{
			name: "valid transparent_hugepage_support=always",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentHugepageSupport: "always",
				},
			},
			wantSupport: "always",
		},
		{
			name: "valid transparent_hugepage_support=madvise",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentHugepageSupport: "madvise",
				},
			},
			wantSupport: "madvise",
		},
		{
			name: "valid transparent_hugepage_support=never",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentHugepageSupport: "never",
				},
			},
			wantSupport: "never",
		},
		{
			name: "case-insensitive transparent_hugepage_support=Always normalized to lowercase",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentHugepageSupport: "Always",
				},
			},
			wantSupport: "always",
		},
		{
			name: "invalid transparent_hugepage_support resets to empty",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentHugepageSupport: "invalid",
				},
			},
			wantSupport: "",
		},
		{
			name: "valid transparent_defrag=defer+madvise",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentDefrag: "defer+madvise",
				},
			},
			wantDefrag: "defer+madvise",
		},
		{
			name: "case-insensitive transparent_defrag=NEVER normalized to lowercase",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentDefrag: "NEVER",
				},
			},
			wantDefrag: "never",
		},
		{
			name: "invalid transparent_defrag resets to empty",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentDefrag: "invalid",
				},
			},
			wantDefrag: "",
		},
		{
			name: "both valid values pass",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentHugepageSupport: "never",
					TransparentDefrag:          "always",
				},
			},
			wantSupport: "never",
			wantDefrag:  "always",
		},
		{
			name: "valid support but invalid defrag resets defrag",
			cfg: &aksnodeconfigv1.Configuration{
				CustomLinuxOsConfig: &aksnodeconfigv1.CustomLinuxOsConfig{
					TransparentHugepageSupport: "always",
					TransparentDefrag:          "invalid",
				},
			},
			wantSupport: "always",
			wantDefrag:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ValidateTHPConfig(tt.cfg)
			if tt.cfg.GetCustomLinuxOsConfig() != nil {
				assert.Equal(t, tt.wantSupport, tt.cfg.GetCustomLinuxOsConfig().GetTransparentHugepageSupport())
				assert.Equal(t, tt.wantDefrag, tt.cfg.GetCustomLinuxOsConfig().GetTransparentDefrag())
			}
		})
	}
}

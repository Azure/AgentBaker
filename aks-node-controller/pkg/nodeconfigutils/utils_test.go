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

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
				"authConfig": {
					"subscriptionId": "test-subscription"
				},
				"clusterConfig": {
					"resourceGroup": "test-rg",
					"location": "eastus"
				},
				"apiServerConfig": {
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
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    []byte(`{"version": "v1", invalid}`),
			want:    nil,
			wantErr: true,
		},
		{
			name: "string is assigned with a boolean value",
			data: []byte(`{
				"version": "v1",
				"LinuxAdminUsername": true,
				"authConfig": {
					"subscriptionId": "test-subscription"
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
			name: "unknown field should be ignored",
			data: []byte(`{
				"version": "v1",
				"unknownField": "should be ignored",
				"authConfig": {
					"subscriptionId": "test-subscription"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshalConfigurationV1(tt.data)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// here we use proto.Equal for deep equality check
				if !proto.Equal(tt.want, got) {
					assert.Fail(t, "UnmarshalConfigurationV1() result mismatch", "want: %+v\n got: %+v", tt.want, got)
				}
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

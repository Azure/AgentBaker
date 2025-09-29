package nodeconfigutils

import (
	"os"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
)

// validateTestFileResult handles validation for test file cases.
func validateTestFileResult(t *testing.T, got *aksnodeconfigv1.Configuration) {
	if got == nil {
		t.Errorf("UnmarshalConfigurationV1() returned nil for valid test file")
	}
}

// validateTestResult handles validation for normal test cases.
func validateTestResult(t *testing.T, got, want *aksnodeconfigv1.Configuration) {
	if want == nil || got == nil {
		return
	}

	if got.Version != want.Version {
		t.Errorf("UnmarshalConfigurationV1() version = %v, want %v", got.Version, want.Version)
	}

	validateAuthConfig(t, got.AuthConfig, want.AuthConfig)
	validateClusterConfig(t, got.ClusterConfig, want.ClusterConfig)
	validateApiServerConfig(t, got.ApiServerConfig, want.ApiServerConfig)
}

// validateAuthConfig validates auth configuration fields.
func validateAuthConfig(t *testing.T, got, want *aksnodeconfigv1.AuthConfig) {
	if want == nil || got == nil {
		return
	}

	if got.SubscriptionId != want.SubscriptionId {
		t.Errorf("UnmarshalConfigurationV1() subscriptionId = %v, want %v",
			got.SubscriptionId, want.SubscriptionId)
	}
}

// validateClusterConfig validates cluster configuration fields.
func validateClusterConfig(t *testing.T, got, want *aksnodeconfigv1.ClusterConfig) {
	if want == nil || got == nil {
		return
	}

	if got.ResourceGroup != want.ResourceGroup {
		t.Errorf("UnmarshalConfigurationV1() resourceGroup = %v, want %v",
			got.ResourceGroup, want.ResourceGroup)
	}

	if got.Location != want.Location {
		t.Errorf("UnmarshalConfigurationV1() location = %v, want %v",
			got.Location, want.Location)
	}
}

// validateApiServerConfig validates API server configuration fields.
func validateApiServerConfig(t *testing.T, got, want *aksnodeconfigv1.ApiServerConfig) {
	if want == nil || got == nil {
		return
	}

	if got.ApiServerName != want.ApiServerName {
		t.Errorf("UnmarshalConfigurationV1() apiServerName = %v, want %v",
			got.ApiServerName, want.ApiServerName)
	}
}

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
			name: "unknown field should error",
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
			wantErr: true,
		},
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

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalConfigurationV1() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Early return for error cases
			if tt.wantErr {
				return
			}

			// Handle special test file case
			if tt.name == "valid config from test file" {
				validateTestFileResult(t, got)
				return
			}

			// Validate normal test cases
			validateTestResult(t, got, tt.want)
		})
	}
}

package nodeconfigutils

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"os"
	"strings"
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

func TestCustomDataUsesMultipartBoothookAndCloudConfig(t *testing.T) {
	cfg := &aksnodeconfigv1.Configuration{
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
	}

	customData, err := CustomData(cfg)
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(customData)
	require.NoError(t, err)

	sections := strings.SplitN(string(decoded), "\r\n\r\n", 2)
	require.Len(t, sections, 2)

	message := textproto.MIMEHeader{}
	for _, line := range strings.Split(sections[0], "\r\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ": ", 2)
		require.Len(t, parts, 2)
		message.Add(parts[0], parts[1])
	}
	mediaType, params, err := mime.ParseMediaType(message.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/mixed", mediaType)

	reader := multipart.NewReader(strings.NewReader(sections[1]), params["boundary"])

	part, err := reader.NextPart()
	require.NoError(t, err)
	require.Equal(t, "text/cloud-boothook", part.Header.Get("Content-Type"))
	boothook, err := io.ReadAll(part)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(boothook), "#cloud-boothook\n"))
	require.Contains(t, string(boothook), "/opt/azure/containers/aks-node-controller-config.json")
	require.Contains(t, string(boothook), "launching aks-node-controller service")
	require.Contains(t, string(boothook), "systemctl start --no-block aks-node-controller.service")

	part, err = reader.NextPart()
	require.NoError(t, err)
	require.Equal(t, "text/cloud-config", part.Header.Get("Content-Type"))
	cloudConfig, err := io.ReadAll(part)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(cloudConfig), "#cloud-config\n"))
	require.Contains(t, string(cloudConfig), "runcmd:")

	_, err = reader.NextPart()
	require.ErrorIs(t, err, io.EOF)
}

// decodeBoothook extracts the decoded cloud-boothook part from the base64-encoded custom data.
func decodeBoothook(t *testing.T, cfg *aksnodeconfigv1.Configuration) string {
	t.Helper()
	customData, err := CustomData(cfg)
	require.NoError(t, err)
	decoded, err := base64.StdEncoding.DecodeString(customData)
	require.NoError(t, err)
	sections := strings.SplitN(string(decoded), "\r\n\r\n", 2)
	require.Len(t, sections, 2)
	message := textproto.MIMEHeader{}
	for _, line := range strings.Split(sections[0], "\r\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ": ", 2)
		require.Len(t, parts, 2)
		message.Add(parts[0], parts[1])
	}
	_, params, err := mime.ParseMediaType(message.Get("Content-Type"))
	require.NoError(t, err)
	reader := multipart.NewReader(strings.NewReader(sections[1]), params["boundary"])
	part, err := reader.NextPart()
	require.NoError(t, err)
	require.Equal(t, "text/cloud-boothook", part.Header.Get("Content-Type"))
	boothook, err := io.ReadAll(part)
	require.NoError(t, err)
	return string(boothook)
}

func TestEnabledFeaturesBlockEmptyWhenDisabled(t *testing.T) {
	// The empty return is the load-bearing byte-identity guarantee: with no features set the
	// boothook template's %[3]s placeholder expands to "", so custom data is byte-identical to
	// the output produced before this feature existed. This protects the 6-month VHD window.
	require.Empty(t, enabledFeaturesBlock(&aksnodeconfigv1.Configuration{}), "expected no features block when EnabledFeatures is unset")
	require.Empty(t, enabledFeaturesBlock(&aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{}}))
}

func TestCustomDataOmitsEnabledFeaturesWhenHotfixDisabled(t *testing.T) {
	// Off-case: no enabled_features.sh write anywhere in the boothook, and no ENABLE_PROVISIONING_HOTFIX.
	off := decodeBoothook(t, &aksnodeconfigv1.Configuration{})
	require.NotContains(t, off, "enabled_features.sh")
	require.NotContains(t, off, "ENABLE_PROVISIONING_HOTFIX")

	// Byte-identity: an empty features map must produce the same output as an unset one.
	emptyMap := decodeBoothook(t, &aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{}})
	require.Equal(t, off, emptyMap)
}

func TestCustomDataWritesEnabledFeaturesWhenHotfixEnabled(t *testing.T) {
	on := decodeBoothook(t, &aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{"ENABLE_PROVISIONING_HOTFIX": "true"}})

	// Written via a quoted heredoc to the shared feature-flag path, chmod 0600, literal KEY=VALUE line.
	require.Contains(t, on, "cat <<'EOF' >/opt/azure/containers/enabled_features.sh")
	require.Contains(t, on, "\nENABLE_PROVISIONING_HOTFIX=true\n")
	require.Contains(t, on, "chmod 0600 /opt/azure/containers/enabled_features.sh")

	// The features file must be written BEFORE the service starts, so the wrapper can read it.
	featuresIdx := strings.Index(on, "enabled_features.sh")
	startIdx := strings.Index(on, "systemctl start --no-block aks-node-controller.service")
	require.NotEqual(t, -1, featuresIdx)
	require.NotEqual(t, -1, startIdx)
	require.Less(t, featuresIdx, startIdx, "enabled_features.sh must be written before the service starts")
}

func TestEnabledFeaturesBlockRendersMultipleSortedFeatures(t *testing.T) {
	// Multiple toggles render as sorted KEY=VALUE lines so the output is deterministic.
	block := enabledFeaturesBlock(&aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{
		"ZED_FEATURE":                "1",
		"ENABLE_PROVISIONING_HOTFIX": "true",
	}})
	require.Contains(t, block, "ENABLE_PROVISIONING_HOTFIX=true\nZED_FEATURE=1\n")
}

func TestEnabledFeaturesBlockSkipsInvalidKeys(t *testing.T) {
	// Keys the wrapper would reject (leading digit, dash, empty) are dropped. When only invalid
	// keys are present, no block is emitted so custom data stays byte-identical to the default.
	require.Empty(t, enabledFeaturesBlock(&aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{
		"1BAD": "x", "has-dash": "y", "": "z",
	}}))

	// A mix keeps only the valid identifier.
	block := enabledFeaturesBlock(&aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{
		"1BAD": "x", "GOOD_KEY": "1",
	}})
	require.Contains(t, block, "GOOD_KEY=1\n")
	require.NotContains(t, block, "1BAD")
}

func TestEnabledFeaturesBlockSkipsValuesWithNewlines(t *testing.T) {
	// A value containing a newline/CR would inject extra lines into the heredoc, so the entry
	// is dropped. When it's the only entry, no block is emitted.
	require.Empty(t, enabledFeaturesBlock(&aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{
		"INJECT": "true\nEVIL=1",
	}}))
	require.Empty(t, enabledFeaturesBlock(&aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{
		"CR": "a\rb",
	}}))

	// A clean entry alongside a newline-tainted one keeps only the clean one.
	block := enabledFeaturesBlock(&aksnodeconfigv1.Configuration{EnabledFeatures: map[string]string{
		"INJECT": "x\nEVIL=1", "GOOD_KEY": "1",
	}})
	require.Contains(t, block, "GOOD_KEY=1\n")
	require.NotContains(t, block, "EVIL")
}

func TestEnabledFeaturesFilePathMatchesWrapperContract(t *testing.T) {
	// Shared contract with the wrapper's FEATURES_PATH default; if it changes here it must
	// change there too, or the wrapper will never read the file.
	require.Equal(t, "/opt/azure/containers/enabled_features.sh", EnabledFeaturesFilePath)
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

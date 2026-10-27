package nodeconfigutils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/stretchr/testify/require"
)

func TestCustomDataFlatcarKubeletFlagsToOmit(test *testing.T) {
	encodedOmissions := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server","--runtime-request-timeout"]`))
	for _, testCase := range []struct {
		name     string
		features map[string]string
		content  string
	}{
		{name: "absent"},
		{name: "empty map", features: map[string]string{}},
		{name: "empty omission request", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": ""}, content: "KUBELET_FLAGS_TO_OMIT=\n"},
		{name: "unrelated valid feature", features: map[string]string{"ENABLE_PROVISIONING_HOTFIX": "true"}},
		{name: "malformed single-line instruction", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": "invalid!"}, content: "KUBELET_FLAGS_TO_OMIT=invalid!\n"},
		{name: "newline", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": encodedOmissions + "\n"}},
		{name: "carriage return", features: map[string]string{"KUBELET_FLAGS_TO_OMIT": encodedOmissions + "\r"}},
		{
			name:     "padded request",
			features: map[string]string{"KUBELET_FLAGS_TO_OMIT": base64.StdEncoding.EncodeToString([]byte(`["--enable-server","--runtime-request-timeout"]`))},
			content:  "KUBELET_FLAGS_TO_OMIT=" + base64.StdEncoding.EncodeToString([]byte(`["--enable-server","--runtime-request-timeout"]`)) + "\n",
		},
		{
			name: "omission request and existing feature",
			features: map[string]string{
				"KUBELET_FLAGS_TO_OMIT":      encodedOmissions,
				"ENABLE_PROVISIONING_HOTFIX": "true",
			},
			content: "KUBELET_FLAGS_TO_OMIT=" + encodedOmissions + "\n",
		},
		{
			name: "invalid entries",
			features: map[string]string{
				"": "empty", "1INVALID": "digit", "HAS-DASH": "dash", "BAD_LINE": "value\nEXTRA=true", "BAD_CR": "value\rEXTRA=true",
			},
		},
		{
			name: "filtering and literal values",
			features: map[string]string{
				"KUBELET_FLAGS_TO_OMIT": encodedOmissions,
				"BAD_LINE":              "value\nEXTRA=true",
				"LITERAL_VALUE":         `$(printf injected); "quoted" \ backtick` + "` value=with=equals",
			},
			content: "KUBELET_FLAGS_TO_OMIT=" + encodedOmissions + "\n",
		},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			config := &aksnodeconfigv1.Configuration{Version: "v1", EnabledFeatures: testCase.features}
			customData, err := CustomDataFlatcar(config)
			require.NoError(test, err)
			payload, err := base64.StdEncoding.DecodeString(customData)
			require.NoError(test, err)
			var ignition struct {
				Ignition struct{ Version string }
				Storage  struct {
					Files []struct {
						Path     string
						Mode     int
						Contents struct{ Source string }
					}
				}
			}
			require.NoError(test, json.Unmarshal(payload, &ignition))
			require.Equal(test, "3.4.0", ignition.Ignition.Version)
			expectedFiles := 1
			if testCase.content != "" {
				expectedFiles++
			}
			require.Len(test, ignition.Storage.Files, expectedFiles)
			configJSON, err := MarshalConfigurationV1(config)
			require.NoError(test, err)
			for fileIndex, file := range ignition.Storage.Files {
				require.Equal(test, 0600, file.Mode)
				require.True(test, strings.HasPrefix(file.Contents.Source, "data:;base64,"))
				content, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(file.Contents.Source, "data:;base64,"))
				require.NoError(test, err)
				if fileIndex == 0 {
					require.Equal(test, AKSNodeConfigFilePath, file.Path)
					require.Equal(test, configJSON, content)
				} else {
					require.Equal(test, EnabledFeaturesFilePath, file.Path)
					require.Equal(test, testCase.content, string(content))
				}
			}
			if testCase.content == "" {
				legacyPayload := fmt.Sprintf(`{
     "ignition": { "version": "3.4.0" },
     "storage": {
       "files": [{
         "path": "/opt/azure/containers/aks-node-controller-config.json",
         "mode": 384,
         "contents": { "source": "data:;base64,%s" }
       }]
     }
    }`, base64.StdEncoding.EncodeToString(configJSON))
				require.Equal(test, legacyPayload, string(payload))
			}
		})
	}
}

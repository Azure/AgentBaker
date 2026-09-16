// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func scriptedPayloadTestConfig() *datamodel.NodeBootstrappingConfiguration {
	profile := &datamodel.AgentPoolProfile{Name: "nodepool1", OSType: datamodel.Linux, Distro: datamodel.AKSUbuntuContainerd2204Gen2}
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Location: "eastus",
			Properties: &datamodel.Properties{
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorVersion: "1.29.0", OrchestratorType: datamodel.Kubernetes,
					KubernetesConfig: &datamodel.KubernetesConfig{ContainerRuntimeConfig: map[string]string{}},
				},
				HostedMasterProfile: &datamodel.HostedMasterProfile{FQDN: "test-cluster.hcp.eastus.azmk8s.io"},
				AgentPoolProfiles:   []*datamodel.AgentPoolProfile{profile},
			},
		},
		AgentPoolProfile: profile,
		CloudSpecConfig:  datamodel.AzurePublicCloudSpecForTest,
		K8sComponents:    &datamodel.K8sComponents{},
		KubeletConfig:    map[string]string{},
	}
}

func TestQuoteCloudConfigFileContent(t *testing.T) {
	for name, content := range map[string]string{
		"empty":         "",
		"no final LF":   "#!/bin/bash\nprintf '%s' \"$HOME\"",
		"final LF":      "#!/bin/bash\ntrue\n",
		"multiple LF":   "text\n\n\n",
		"whitespace":    "\tleading tab\n  leading spaces\ntrailing spaces  \n",
		"YAML syntax":   "#cloud-config\n---\n- &anchor !tag\nkey: \"quoted\" '\\' # comment\n...\n",
		"shell syntax":  "cat <<'EOF'\n${VALUE}\n$(false)\n`false`\nEOF\n",
		"CRLF":          "first\r\nsecond\r\n",
		"UTF-8":         "\u00e9 \u2603 \U0001f600 \u0085 \u2028 \u2029 \ufeff\n",
		"control bytes": "\x00\x01\x1b\x7f\n",
		"binary":        "\x00\xff\x80\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			const configTemplate = `#cloud-config
bootcmd:
- [bash, -c, "echo unchanged"]
write_files:
- path: /opt/test
  permissions: "0755"
  owner: root:root
  append: true
  defer: true
  encoding: %s
  content: %s
- path: /opt/base64
  encoding: base64
  content: dGV4dA==
- path: /opt/plain
  content: "untouched\n"
runcmd:
- echo done
packages: [curl]
`
			input := fmt.Sprintf(configTemplate, "gzip", "!!binary "+getBase64EncodedGzippedCustomScriptFromStr(content))
			quoted, err := quoteCloudConfigFileContent(content)
			require.NoError(t, err)
			output := fmt.Sprintf(configTemplate, `""`, quoted)
			require.True(t, strings.HasPrefix(output, "#cloud-config\n"))
			require.Equal(t, normalizedCloudConfig(t, input), normalizedCloudConfig(t, output))
			var decoded struct {
				WriteFiles []cloudInitWriteFile `yaml:"write_files"`
			}
			require.NoError(t, yaml.Unmarshal([]byte(output), &decoded))
			require.Empty(t, decoded.WriteFiles[0].Encoding)
			require.Equal(t, content, decoded.WriteFiles[0].Content)
		})
	}
}

func FuzzQuoteCloudConfigFileContent(f *testing.F) {
	for _, content := range []string{"", "line\n", "\tindent\r\n", "\x00\xff\x80", "\u0085\u2028\u2029", "\"'\\${VALUE}"} {
		f.Add(content)
	}
	f.Fuzz(func(t *testing.T, content string) {
		quoted, err := quoteCloudConfigFileContent(content)
		require.NoError(t, err)
		var decoded string
		require.NoError(t, yaml.Unmarshal([]byte(quoted), &decoded))
		require.Equal(t, content, decoded)
	})
}

// Normalize only the file encoding, retaining every other field and file order.
func normalizedCloudConfig(t require.TestingT, input string) map[string]interface{} {
	var config map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(input), &config))
	files, ok := config["write_files"].([]interface{})
	require.True(t, ok, "write_files must be a sequence")
	for _, value := range files {
		file, ok := value.(map[string]interface{})
		require.True(t, ok, "write_files entries must be mappings")
		switch file["encoding"] {
		case encodingGZIP:
			compressed, ok := file["content"].(string)
			require.True(t, ok, "gzip content must be a decoded binary string")
			content, err := getGzipDecodedValue([]byte(compressed))
			require.NoError(t, err)
			file["content"] = string(content)
			delete(file, "encoding")
		case "":
			delete(file, "encoding")
		}
	}
	return config
}

func TestScriptedCloudConfigPayloadCompatibility(t *testing.T) {
	generator := InitializeTemplateGenerator()
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2, datamodel.AKSUbuntuContainerd2404Gen2,
		datamodel.AKSUbuntuMinimalContainerd2604Gen2, datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSAzureLinuxV3Gen2FIPS, datamodel.AKSAzureLinuxV3OSGuardGen2FIPSTL,
		datamodel.AKSACLGen2TL, datamodel.AKSFlatcarGen2,
	} {
		for _, customCloud := range []bool{false, true} {
			for _, preProvisionOnly := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/customCloud=%t/preProvisionOnly=%t", distro, customCloud, preProvisionOnly), func(t *testing.T) {
					config := scriptedPayloadTestConfig()
					config.AgentPoolProfile.Distro = distro
					config.PreProvisionOnly = preProvisionOnly
					if customCloud {
						config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
							Name: "akscustom", ResourceManagerEndpoint: "https://management.fixture.invalid/",
						}
					}
					payload := generator.getLinuxNodeBootstrappingPayload(config)
					wire, err := base64.StdEncoding.DecodeString(payload)
					require.NoError(t, err)
					if config.IsFlatcar() || config.IsACL() {
						expected := getCustomDataFromJSON(generator.getFlatcarLinuxNodeCustomDataJSONObject(config))
						require.Equal(t, expected, string(wire), "Ignition delivery must not change")
						t.Logf("Unchanged Ignition: %d decoded bytes, %d encoded chars", len(wire), len(payload))
					} else {
						require.LessOrEqual(t, len(payload), MaxCustomDataLength)
						require.LessOrEqual(t, len(wire), 65535)
						t.Logf("CustomData: %d decoded bytes, %d encoded chars, %d chars headroom",
							len(wire), len(payload), MaxCustomDataLength-len(payload))
						content, err := getGzipDecodedValue(wire)
						require.NoError(t, err)
						require.True(t, strings.HasPrefix(string(content), "#cloud-config\n"))
						expected := getCustomDataFromJSON(generator.getLinuxNodeCustomDataJSONObject(config))
						require.Equal(t, normalizedCloudConfig(t, expected), normalizedCloudConfig(t, string(content)))
						require.Equal(t, payload, generator.getLinuxNodeBootstrappingPayload(config))
					}
				})
			}
		}
	}
}

func TestCloudConfigScriptlessPayloadUnchanged(t *testing.T) {
	generator := InitializeTemplateGenerator()
	for _, nbc := range []bool{false, true} {
		for _, preProvisionOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("nbc=%t/preProvisionOnly=%t", nbc, preProvisionOnly), func(t *testing.T) {
				newConfig := func() *datamodel.NodeBootstrappingConfiguration {
					config := scriptedPayloadTestConfig()
					config.EnableScriptlessCSECmd = true
					config.EnableScriptlessNBCCSECmd = nbc
					config.PreProvisionOnly = preProvisionOnly
					config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{
						Name: "akscustom", ResourceManagerEndpoint: "https://management.fixture.invalid/",
					}
					return config
				}
				var expected string
				if nbc && !preProvisionOnly {
					expected = generator.getScriptlessBoothook(newConfig())
				} else {
					data := getCustomDataFromJSON(generator.getLinuxNodeCustomDataJSONObject(newConfig()))
					expected = getBase64EncodedGzippedCustomScriptFromStr(data)
				}
				require.Equal(t, expected, generator.getLinuxNodeBootstrappingPayload(newConfig()))
			})
		}
	}
}

func BenchmarkScriptedCloudConfigPayload(b *testing.B) {
	generator := InitializeTemplateGenerator()
	config := scriptedPayloadTestConfig()
	b.Run("nested-gzip", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			data := getCustomDataFromJSON(generator.getLinuxNodeCustomDataJSONObject(config))
			getBase64EncodedGzippedCustomScriptFromStr(data)
		}
	})
	b.Run("shared-gzip", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			generator.getLinuxNodeBootstrappingPayload(config)
		}
	})
}

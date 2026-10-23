package agent

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"text/template"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestTraditionalCloudInitUsesSingleCompression(t *testing.T) {
	template, err := parts.Templates.ReadFile(kubernetesNodeCustomDataYaml)
	require.NoError(t, err)
	for _, distro := range []datamodel.Distro{datamodel.AKSUbuntuContainerd2204Gen2, datamodel.AKSAzureLinuxV2Gen2, datamodel.AKSAzureLinuxV3Gen2} {
		for _, custom := range []bool{false, true} {
			for _, dns := range []bool{false, true} {
				for _, preProvision := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/custom=%t/localdns=%t/preProvision=%t", distro, custom, dns, preProvision), func(t *testing.T) {
						config := newNodeCustomDataRenderConfig(distro)
						config.PreProvisionOnly = preProvision
						config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{EnableLocalDNS: dns}
						if custom {
							config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
						}
						assertTraditionalCloudInitPayload(t, template, config)
					})
				}
			}
		}
	}
}

func assertTraditionalCloudInitPayload(t *testing.T, template []byte, config *datamodel.NodeBootstrappingConfiguration) {
	t.Helper()
	generator := InitializeTemplateGenerator()
	original, err := generator.getSingleLine(kubernetesNodeCustomDataYaml, config.AgentPoolProfile, legacyCloudInitFuncMap(config), true)
	require.NoError(t, err)
	payload := generator.getLinuxNodeBootstrappingPayload(config)
	compressed, err := base64.StdEncoding.Strict().DecodeString(payload)
	require.NoError(t, err)
	decoded, err := getGzipDecodedValue(compressed)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(decoded), "#cloud-config\n"))
	rendered, err := RenderLinuxNodeCustomDataTemplate(template, config)
	require.NoError(t, err)
	require.Equal(t, rendered, string(decoded))
	var before, after cloudInit
	require.NoError(t, yaml.Unmarshal([]byte(original), &before))
	require.NoError(t, yaml.Unmarshal(decoded, &after))
	beforeFiles, err := buildIgnitionTarEntries(before)
	require.NoError(t, err)
	afterFiles, err := buildIgnitionTarEntries(after)
	require.NoError(t, err)
	require.Equal(t, beforeFiles, afterFiles)
	for _, file := range after.WriteFiles {
		require.Empty(t, file.Encoding)
	}
	require.Less(t, len(payload), len(getBase64EncodedGzippedCustomScriptFromStr(original)))
	t.Logf("customData: %d encoded characters", len(payload))
	// Ubuntu custom-cloud overflow is tracked separately in #9790.
	if config.ContainerService.Properties.CustomCloudEnv == nil || config.AgentPoolProfile.Distro != datamodel.AKSUbuntuContainerd2204Gen2 {
		require.Less(t, len(payload), MaxCustomDataLength)
	}
}

func TestCloudInitEncodingLeavesOtherModesUnchanged(t *testing.T) {
	template, err := parts.Templates.ReadFile(kubernetesNodeCustomDataYaml)
	require.NoError(t, err)
	generator := InitializeTemplateGenerator()
	for _, distro := range []datamodel.Distro{datamodel.AKSUbuntuContainerd2204Gen2, datamodel.AKSAzureLinuxV3Gen2, datamodel.AKSFlatcarGen2, datamodel.AKSACLGen2TL} {
		for _, scriptless := range []bool{false, true} {
			if !scriptless && distro != datamodel.AKSFlatcarGen2 && distro != datamodel.AKSACLGen2TL {
				continue
			}
			t.Run(fmt.Sprintf("%s/scriptless=%t", distro, scriptless), func(t *testing.T) {
				config := newNodeCustomDataRenderConfig(distro)
				config.EnableScriptlessCSECmd = scriptless
				config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
				original, err := generator.getSingleLine(kubernetesNodeCustomDataYaml, config.AgentPoolProfile, legacyCloudInitFuncMap(config), true)
				require.NoError(t, err)
				rendered, err := RenderLinuxNodeCustomDataTemplate(template, config)
				require.NoError(t, err)
				require.Equal(t, original, rendered)
				require.Equal(t, original, getCustomDataFromJSON(generator.getLinuxNodeCustomDataJSONObject(config)))
			})
		}
	}
}

func legacyCloudInitFuncMap(config *datamodel.NodeBootstrappingConfiguration) template.FuncMap {
	funcs := getBakerFuncMap(config, getParameters(config), getCustomDataVariablesWithEncoder(config, false))
	funcs["GetCloudInitFileEncoding"] = func() string { return encodingGZIP }
	funcs["GetCloudInitFileContent"] = func(content string) string { return "!!binary |\n    " + content }
	return funcs
}

func TestCloudInitRenderingPreservesAdditionalFields(t *testing.T) {
	input := []byte("#cloud-config\nruncmd:\n- echo hello\nwrite_files:\n- path: /opt/example\n  append: true\n  defer: true\n  content: \"a\\tb\\n\\n\"\n")
	output, err := RenderLinuxNodeCustomDataTemplate(input, newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2))
	require.NoError(t, err)
	require.Equal(t, string(input), output)
}

func TestPlainCloudInitFileContentPreservesWhitespace(t *testing.T) {
	input := []byte("#cloud-config\nwrite_files:\n- path: /opt/example\n  encoding: {{GetCloudInitFileEncoding}}\n  content: {{.Name | GetCloudInitFileContent}}\n")
	for _, content := range []string{"tab\tquote\"backslash\\", "trailing newline\n\n", "\u00e9\r\n"} {
		config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
		config.AgentPoolProfile.Name = content
		output, err := RenderLinuxNodeCustomDataTemplate(input, config)
		require.NoError(t, err)
		var parsed cloudInit
		require.NoError(t, yaml.Unmarshal([]byte(output), &parsed))
		require.Len(t, parsed.WriteFiles, 1)
		require.Empty(t, parsed.WriteFiles[0].Encoding)
		require.Equal(t, content, parsed.WriteFiles[0].Content)
	}
}

func BenchmarkTraditionalCloudInitEncoding(b *testing.B) {
	for _, old := range []bool{true, false} {
		b.Run(fmt.Sprintf("legacy=%t", old), func(b *testing.B) {
			config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
			generator := InitializeTemplateGenerator()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if old {
					rendered, err := generator.getSingleLineForTemplate(kubernetesNodeCustomDataYaml, config.AgentPoolProfile, legacyCloudInitFuncMap(config), true)
					if err != nil {
						b.Fatal(err)
					}
					customData := getCustomDataFromJSON(fmt.Sprintf("{\"customData\": \"%s\"}", rendered))
					_ = getBase64EncodedGzippedCustomScriptFromStr(customData)
				} else {
					_ = generator.getLinuxNodeBootstrappingPayload(config)
				}
			}
		})
	}
}

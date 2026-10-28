// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

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
	input, err := parts.Templates.ReadFile(kubernetesNodeCustomDataYaml)
	require.NoError(t, err)
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSUbuntuContainerd2404Gen2,
		datamodel.AKSAzureLinuxV2Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
	} {
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
						assertTraditionalCloudInitPayload(t, input, config)
					})
				}
			}
		}
	}
}

func assertTraditionalCloudInitPayload(t *testing.T, input []byte, config *datamodel.NodeBootstrappingConfiguration) {
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
	rendered, err := RenderLinuxNodeCustomDataTemplate(input, config)
	require.NoError(t, err)
	require.Equal(t, rendered, string(decoded))
	var before, after cloudInit
	require.NoError(t, yaml.Unmarshal([]byte(original), &before))
	require.NoError(t, yaml.Unmarshal(decoded, &after))
	var document struct {
		WriteFiles []map[string]any `yaml:"write_files"`
	}
	require.NoError(t, yaml.Unmarshal(decoded, &document))
	for _, file := range document.WriteFiles {
		_, hasEncoding := file["encoding"]
		require.False(t, hasEncoding, "plain files must omit encoding rather than use an empty value")
	}
	beforeFiles, err := buildIgnitionTarEntries(before)
	require.NoError(t, err)
	afterFiles, err := buildIgnitionTarEntries(after)
	require.NoError(t, err)
	require.Equal(t, beforeFiles, afterFiles)
	beforePayload := getBase64EncodedGzippedCustomScriptFromStr(original)
	require.Less(t, len(payload), len(beforePayload))
	t.Logf("customData: before=%d after=%d limit=%d", len(beforePayload), len(payload), MaxCustomDataLength)
	// Ubuntu custom-cloud overflow is tracked separately in #9790.
	isUbuntu := config.AgentPoolProfile.Distro == datamodel.AKSUbuntuContainerd2204Gen2 ||
		config.AgentPoolProfile.Distro == datamodel.AKSUbuntuContainerd2404Gen2
	if config.ContainerService.Properties.CustomCloudEnv == nil || !isUbuntu {
		require.Less(t, len(payload), MaxCustomDataLength)
	}
}

func TestCloudInitEncodingLeavesOtherModesUnchanged(t *testing.T) {
	input, err := parts.Templates.ReadFile(kubernetesNodeCustomDataYaml)
	require.NoError(t, err)
	generator := InitializeTemplateGenerator()
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSFlatcarGen2,
		datamodel.AKSACLGen2TL,
	} {
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
				rendered, err := RenderLinuxNodeCustomDataTemplate(input, config)
				require.NoError(t, err)
				require.Equal(t, original, rendered)
				require.Equal(t, original, getCustomDataFromJSON(generator.getLinuxNodeCustomDataJSONObject(config)))
			})
		}
	}
}

func TestTraditionalCloudInitCustomImages(t *testing.T) {
	input, err := parts.Templates.ReadFile(kubernetesNodeCustomDataYaml)
	require.NoError(t, err)
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2004CVMGen2,
		datamodel.CustomizedImage,
		datamodel.CustomizedImageLinuxGuard,
		datamodel.CustomizedImageTrustedLaunch,
	} {
		t.Run(string(distro), func(t *testing.T) {
			assertTraditionalCloudInitPayload(t, input, newNodeCustomDataRenderConfig(distro))
		})
	}
}

func TestCloudInitLegacyAndQuotedEntries(t *testing.T) {
	input := []byte(`#cloud-config
write_files:
- path: /opt/legacy.sh
  permissions: "0744"
  owner: root
  encoding: gzip
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSource"}}
- path: /opt/quoted.sh
  permissions: "0744"
  owner: root
  {{GetCloudInitFileEncodingLine}}
  content: {{GetVariableProperty "cloudInitFileData" "provisionSource" | GetCloudInitFileContent}}
`)
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSFlatcarGen2,
		datamodel.AKSACLGen2TL,
	} {
		for _, scriptless := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/scriptless=%t", distro, scriptless), func(t *testing.T) {
				config := newNodeCustomDataRenderConfig(distro)
				config.EnableScriptlessCSECmd = scriptless
				rendered, err := RenderLinuxNodeCustomDataTemplate(input, config)
				require.NoError(t, err)
				var cloud cloudInit
				require.NoError(t, yaml.Unmarshal([]byte(rendered), &cloud))
				files, err := buildIgnitionTarEntries(cloud)
				require.NoError(t, err)
				require.Len(t, files, 2)
				require.Equal(t, []byte(getCustomScript(kubernetesCSEHelpersScript, config)), files[0].contents)
				require.Equal(t, files[0].contents, files[1].contents)
				require.Equal(t, encodingGZIP, cloud.WriteFiles[0].Encoding)
				if usePlainCloudInit(config) {
					require.Empty(t, cloud.WriteFiles[1].Encoding)
				} else {
					require.Equal(t, encodingGZIP, cloud.WriteFiles[1].Encoding)
				}
			})
		}
	}
}

func legacyCloudInitFuncMap(config *datamodel.NodeBootstrappingConfiguration) template.FuncMap {
	funcs := getBakerFuncMap(config, getParameters(config), getCustomDataVariablesWithEncoder(config, false))
	funcs["GetCloudInitFileEncodingLine"] = func() string { return "encoding: " + encodingGZIP }
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
	input := []byte(`#cloud-config
write_files:
- path: /opt/example
  {{GetCloudInitFileEncodingLine}}
  content: {{.Name | GetCloudInitFileContent}}
`)
	for _, content := range []string{
		"",
		" \tleading and trailing \t ",
		"tab\tquote\"backslash\\",
		"trailing newline\n\n",
		"\u00e9\r\n",
		"next\u0085line\u2028paragraph\u2029end",
		"nul\x00escape\x1b",
		"echo \"$HOME\"; printf '%s\\n' 'a: # & < >'\n",
		strings.Repeat("long line: # \"\\ ", 1024),
	} {
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

func TestPlainCloudInitFileContentRejectsInvalidUTF8(t *testing.T) {
	input := []byte("#cloud-config\nwrite_files:\n- path: /opt/example\n  content: {{.Name | GetCloudInitFileContent}}\n")
	config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
	config.AgentPoolProfile.Name = string([]byte{'a', 0xff, 'b'})
	output, err := RenderLinuxNodeCustomDataTemplate(input, config)
	require.ErrorContains(t, err, "invalid UTF-8")
	require.Empty(t, output)
}

package agent

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPOSIXCksumMatchesSystem(t *testing.T) {
	if _, err := exec.LookPath("cksum"); err != nil {
		t.Skip("cksum is not installed")
	}
	for _, content := range []string{
		"",
		"#!/bin/bash\nprintf 'hello\\n'\n",
		"\x00\xff\xc3\xa9\r\n\u0085\u2028\u2029",
		strings.Repeat("x", 255),
		strings.Repeat("x", 256),
		strings.Repeat("x", 65536),
	} {
		cmd := exec.Command("cksum")
		cmd.Stdin = strings.NewReader(content)
		output, err := cmd.Output()
		require.NoError(t, err)
		require.Equal(t, string(output), fmt.Sprintf("%d %d\n", posixCksum(content), len(content)))
	}
}

func TestCloudInitChecksumsMatchDeliveredFiles(t *testing.T) {
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSUbuntuContainerd2404Gen2,
		datamodel.AKSAzureLinuxV2Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSAzureLinuxV3OSGuardGen2FIPSTL,
		datamodel.CustomizedImage,
		datamodel.CustomizedImageLinuxGuard,
		datamodel.CustomizedImageTrustedLaunch,
	} {
		for _, custom := range []bool{false, true} {
			for _, preProvision := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/custom=%t/preProvision=%t", distro, custom, preProvision), func(t *testing.T) {
					config := newNodeCustomDataRenderConfig(distro)
					config.PreProvisionOnly = preProvision
					if custom {
						config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
					}
					generator := InitializeTemplateGenerator()
					payload := generator.getLinuxNodeBootstrappingPayload(config)
					compressed, err := base64.StdEncoding.DecodeString(payload)
					require.NoError(t, err)
					decoded, err := getGzipDecodedValue(compressed)
					require.NoError(t, err)
					var cloud cloudInit
					require.NoError(t, yaml.Unmarshal(decoded, &cloud))
					var expected []string
					for _, file := range cloud.WriteFiles {
						expected = append(expected, shellQuote(fmt.Sprintf("%d %d %s", posixCksum(file.Content), len(file.Content), file.Path)))
					}
					checksums, err := getCloudInitFileChecksums(config)
					require.NoError(t, err)
					require.NotEmpty(t, checksums)
					require.Equal(t, strings.Join(expected, " "), checksums)
					assertCloudInitChecksumCommand(t, generator.getLinuxNodeCSECommand(config), checksums, cloud)
				})
			}
		}
	}
}

func assertCloudInitChecksumCommand(t *testing.T, command, checksums string, cloud cloudInit) {
	t.Helper()
	require.Contains(t, command, "for checksum in "+checksums+"; do")
	require.Less(t, strings.Index(command, "for checksum in "), strings.Index(command, "INIT_AKS_CLOUD_FILEPATH="))
	if _, err := exec.LookPath("sh"); err != nil {
		t.Log("sh is not installed; skipping shell execution")
		return
	}
	output, err := exec.Command("sh", "-n", "-c", command).CombinedOutput()
	require.NoError(t, err, "%s", output)
	if _, lookupErr := exec.LookPath("cksum"); lookupErr != nil {
		t.Log("cksum is not installed; skipping checksum command execution")
		return
	}
	_, check, found := strings.Cut(command, "for checksum in ")
	require.True(t, found)
	check, _, found = strings.Cut(check, "done;")
	require.True(t, found)
	check = "for checksum in " + check + "done;"
	root := t.TempDir()
	for _, file := range cloud.WriteFiles {
		target := filepath.Join(root, strings.TrimPrefix(file.Path, "/"))
		require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o700))
		require.NoError(t, os.WriteFile(target, []byte(file.Content), 0o600))
		check = strings.ReplaceAll(check, file.Path, target)
	}
	cmd := exec.Command("sh", "-c", check)
	cmd.Env = append(os.Environ(), "PROVISION_OUTPUT="+filepath.Join(root, "provision.log"))
	output, err = cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
	require.Empty(t, output)
}

func TestCloudInitChecksumsSkipOtherModes(t *testing.T) {
	for _, mode := range []string{"disabled", "scriptless", "flatcar", "acl"} {
		t.Run(mode, func(t *testing.T) {
			config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
			switch mode {
			case "disabled":
				config.DisableCustomData = true
			case "scriptless":
				config.EnableScriptlessCSECmd = true
			case "flatcar":
				config.AgentPoolProfile.Distro = datamodel.AKSFlatcarGen2
			case "acl":
				config.AgentPoolProfile.Distro = datamodel.AKSACLGen2TL
			}
			checksums, err := getCloudInitFileChecksums(config)
			require.NoError(t, err)
			require.Empty(t, checksums)
			require.NotContains(t, InitializeTemplateGenerator().getLinuxNodeCSECommand(config), "for checksum in ")
		})
	}
}

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
	var document struct {
		WriteFiles []map[string]any `yaml:"write_files"`
	}
	require.NoError(t, yaml.Unmarshal(decoded, &document))
	for _, file := range document.WriteFiles {
		_, hasEncoding := file["encoding"]
		require.False(t, hasEncoding, "plain files must use cloud-init's default, not an invalid empty encoding")
	}
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
	require.LessOrEqual(t, len(payload), MaxCustomDataLength)
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

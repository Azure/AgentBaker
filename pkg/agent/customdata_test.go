package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLinuxCustomDataSize(t *testing.T) {
	for _, distro := range []datamodel.Distro{
		datamodel.AKSAzureLinuxV2Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSCBLMarinerV2Gen2,
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSUbuntuContainerd2404Gen2,
		datamodel.AKSAzureLinuxV3OSGuardGen2FIPSTL,
	} {
		t.Run(string(distro), func(t *testing.T) {
			config := newNodeCustomDataRenderConfig(distro)
			config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{EnableLocalDNS: true}
			generator := InitializeTemplateGenerator()
			original := getCustomDataFromJSON(generator.getLinuxNodeCustomDataJSONObject(config))
			customData := generator.getLinuxNodeBootstrappingPayload(config)
			decoded, err := base64.StdEncoding.DecodeString(customData)
			require.NoError(t, err)
			t.Logf("encoded customData: %d characters (previously %d), decoded: %d bytes",
				len(customData), len(getBase64EncodedGzippedCustomScriptFromStr(original)), len(decoded))
			require.LessOrEqual(t, len(customData), MaxCustomDataLength)
			require.LessOrEqual(t, len(decoded), 65535)
			rendered, err := getGzipDecodedValue(decoded)
			require.NoError(t, err)
			require.Equal(t, effectiveCloudConfig(t, original), effectiveCloudConfig(t, string(rendered)))
		})
	}
}

func effectiveCloudConfig(t *testing.T, content string) cloudInit {
	t.Helper()
	var config cloudInit
	require.NoError(t, yaml.Unmarshal([]byte(content), &config))
	for i := range config.WriteFiles {
		file := &config.WriteFiles[i]
		if file.Encoding == encodingGZIP {
			decoded, err := getGzipDecodedValue([]byte(file.Content))
			require.NoError(t, err)
			file.Content = string(decoded)
			file.Encoding = ""
		}
	}
	return config
}

func TestCompressedCloudConfigPreservesContents(t *testing.T) {
	for _, content := range []string{
		"#!/bin/bash\nprintf '%s\\n' 'hello: # world'\n\n",
		"no trailing newline",
		"",
		"\x00\xffbinary\x80",
	} {
		t.Run(fmt.Sprintf("%q", content), func(t *testing.T) {
			original := fmt.Sprintf(`#cloud-config
bootcmd:
- [echo, boot]
runcmd:
- echo done
write_files:
- path: /opt/test
  permissions: "0744"
  owner: root:root
  append: true
  defer: true
  encoding: gzip
  content: !!binary %s
- path: /opt/plain
  content: |
    unchanged
- path: /opt/base64
  encoding: base64
  content: dW5jaGFuZ2Vk
`, getBase64EncodedGzippedCustomScriptFromStr(content))
			encoded, err := getCompressedCloudConfig(original)
			require.NoError(t, err)
			compressed, err := base64.StdEncoding.DecodeString(encoded)
			require.NoError(t, err)
			rendered, err := getGzipDecodedValue(compressed)
			require.NoError(t, err)
			require.True(t, bytes.HasPrefix(rendered, []byte("#cloud-config\n")))

			var before, after map[string]interface{}
			require.NoError(t, yaml.Unmarshal([]byte(original), &before))
			require.NoError(t, yaml.Unmarshal(rendered, &after))
			files, ok := before["write_files"].([]interface{})
			require.True(t, ok)
			file, ok := files[0].(map[string]interface{})
			require.True(t, ok)
			file["content"] = content
			file["encoding"] = ""
			require.Equal(t, before, after)
		})
	}
}

func TestCompressedCloudConfigErrors(t *testing.T) {
	for _, tc := range []struct {
		name, input, wantErr string
	}{
		{"invalid YAML", "write_files: [", "unmarshal cloud-config"},
		{"invalid write_files", "write_files: {}", "write_files must be a sequence"},
		{"invalid file entry", "write_files: [not-a-file]", "decode write_files entry"},
		{"invalid gzip", "write_files:\n- path: /opt/test\n  encoding: gzip\n  content: not-gzip", "decode gzip content for /opt/test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := getCompressedCloudConfig(tc.input)
			require.Empty(t, result)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestCompressedCloudConfigWithoutGzip(t *testing.T) {
	const original = `#cloud-config
write_files:
- path: /opt/azure/containers/scriptless-cse-overrides.txt
  permissions: "0644"
  owner: root
  content: |
    Executing in scriptless CSE mode.
`
	result, err := getCompressedCloudConfig(original)
	require.NoError(t, err)
	require.Equal(t, getBase64EncodedGzippedCustomScriptFromStr(original), result)
}

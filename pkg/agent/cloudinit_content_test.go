package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCloudInitFileContentRoundTrip(test *testing.T) {
	cases := map[string]string{
		"empty":             "",
		"no final newline":  "printf '%s' done",
		"one final newline": "printf '%s' done\n",
		"blank lines":       "\n\nfirst\n\nlast\n\n\n",
		"indentation":       "  leading\n\tindented\n    nested\n",
		"shell and YAML":    "#AKS marker\necho \"${value:-a:b} #literal\"\n---\n...\n&anchor *alias !tag {key: value}\n",
		"heredoc":           "cat <<'EOF'\n  spaces\n\ttabs\n${literal} \\n {{literal}}\nEOF\n",
		"control bytes":     "\x00\a\b\f\r\n\t\v\x1b\x7f",
		"unicode":           "caf\u00e9 \u4e2d\u6587 \U0001f916\nnext\u0085line\u2028separator\u2029paragraph\n",
		"invalid UTF8":      string([]byte{0xff, 0xfe, 0x80, '\n', 0xc3, 0x28}),
	}
	for name, content := range cases {
		test.Run(name, func(test *testing.T) {
			scalar, err := getCloudInitFileContent(getBase64EncodedGzippedCustomScriptFromStr(content))
			require.NoError(test, err)
			document := "write_files:\n- path: /etc/example\n  permissions: \"0600\"\n  owner: root\n  content: " + scalar +
				"\n- path: /etc/next\n  permissions: \"0644\"\n  owner: root\n  content: next\n"
			transported := getCustomDataFromJSON("{\"customData\":\"" + escapeSingleLine(document) + "\"}")
			var decoded cloudInit
			require.NoError(test, yaml.Unmarshal([]byte(transported), &decoded))
			require.Len(test, decoded.WriteFiles, 2)
			require.Equal(test, content, decoded.WriteFiles[0].Content, "scalar=%q", scalar)
			require.Empty(test, decoded.WriteFiles[0].Encoding)
			entries, err := buildIgnitionTarEntries(decoded)
			require.NoError(test, err)
			require.Equal(test, []ignitionTarEntry{
				{path: "/etc/example", mode: 0o600, owner: "root", contents: []byte(content)},
				{path: "/etc/next", mode: 0o644, owner: "root", contents: []byte("next")},
			}, entries)
		})
	}
}

func TestCloudInitFileContentRejectsInvalidEncoding(test *testing.T) {
	compressed, err := base64.StdEncoding.DecodeString(getBase64EncodedGzippedCustomScriptFromStr("content"))
	require.NoError(test, err)
	for name, encoded := range map[string]string{
		"invalid base64": "not base64!",
		"empty gzip":     "",
		"not gzip":       base64.StdEncoding.EncodeToString([]byte("plain text")),
		"truncated gzip": base64.StdEncoding.EncodeToString(compressed[:len(compressed)-1]),
	} {
		test.Run(name, func(test *testing.T) {
			content, err := getCloudInitFileContent(encoded)
			require.Error(test, err)
			require.Empty(test, content)
		})
	}
}

func TestCloudInitTraditionalFileParity(test *testing.T) {
	for _, platform := range []struct {
		distro datamodel.Distro
		suffix string
	}{
		{datamodel.AKSUbuntuContainerd2204Gen2, "Ubuntu"},
		{datamodel.AKSAzureLinuxV2Gen2, "Mariner"},
		{datamodel.AKSAzureLinuxV3Gen2, "Mariner"},
		{datamodel.AKSAzureLinuxV3OSGuardGen2FIPSTL, "AzlOSGuard"},
		{datamodel.AKSFlatcarGen2, "Flatcar"},
		{datamodel.AKSACLGen2TL, "ACL"},
	} {
		for _, version := range []string{"1.29.0", "1.38.0"} {
			for _, configEnabled := range []bool{false, true} {
				for _, omissionRequested := range []bool{false, true} {
					name := fmt.Sprintf("%s/%s/config=%t/omission=%t", platform.distro, version, configEnabled, omissionRequested)
					test.Run(name, func(test *testing.T) {
						config := newNodeCustomDataRenderConfig(platform.distro)
						config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = version
						config.EnableKubeletConfigFile = configEnabled
						if omissionRequested {
							config.EnabledFeatures = map[string]string{
								"KUBELET_FLAGS_TO_OMIT": base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server"]`)),
							}
						}
						assertCloudInitTraditionalFileParity(test, config, platform.suffix, true)
					})
				}
			}
		}
	}
}

func TestCloudInitCustomCloudAndImageFileParity(test *testing.T) {
	for _, customImage := range []bool{false, true} {
		test.Run(fmt.Sprintf("customImage=%t", customImage), func(test *testing.T) {
			config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
			config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
			if customImage {
				config.AgentPoolProfile.Distro = datamodel.CustomizedImage
			}
			assertCloudInitTraditionalFileParity(test, config, "Ubuntu", !customImage)
		})
	}
}

func TestCloudInitScriptlessPreservesGzipFile(test *testing.T) {
	config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
	config.EnableScriptlessCSECmd = true
	config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
	variables, ok := getCustomDataVariables(config)["cloudInitData"].(paramsMap)
	require.True(test, ok)
	encoded, ok := variables["initAKSCloud"].(string)
	require.True(test, ok)
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(test, err)
	var decoded cloudInit
	generator := InitializeTemplateGenerator()
	document := getCustomDataFromJSON(generator.getLinuxNodeCustomDataJSONObject(config))
	require.NoError(test, yaml.Unmarshal([]byte(document), &decoded))
	require.Len(test, decoded.WriteFiles, 2)
	require.Equal(test, "/opt/azure/containers/scriptless-cse-overrides.txt", decoded.WriteFiles[0].Path)
	require.Equal(test, cloudInitWriteFile{
		Path: initAKSCloudFilepath, Permissions: "0744", Owner: "root", Encoding: encodingGZIP, Content: string(compressed),
	}, decoded.WriteFiles[1])
}

func assertCloudInitTraditionalFileParity(test *testing.T, config *datamodel.NodeBootstrappingConfiguration, distroSuffix string, includeStart bool) {
	test.Helper()
	variables, ok := getCustomDataVariables(config)["cloudInitData"].(paramsMap)
	require.True(test, ok)
	expectedFiles := expectedCloudInitTraditionalFiles(test, variables, distroSuffix, includeStart, config.ContainerService.IsAKSCustomCloud())
	generator := InitializeTemplateGenerator()
	document := getCustomDataFromJSON(generator.getLinuxNodeCustomDataJSONObject(config))
	var decoded cloudInit
	require.NoError(test, yaml.Unmarshal([]byte(document), &decoded))
	require.Len(test, decoded.WriteFiles, len(expectedFiles))
	seenPaths := make(map[string]bool, len(expectedFiles))
	for _, file := range decoded.WriteFiles {
		require.NotContains(test, seenPaths, file.Path)
		seenPaths[file.Path] = true
		expected, exists := expectedFiles[file.Path]
		require.True(test, exists, "unexpected file %s", file.Path)
		require.Equal(test, fmt.Sprintf("%04o", expected.mode), file.Permissions, file.Path)
		require.Equal(test, expected.owner, file.Owner, file.Path)
		content := []byte(file.Content)
		if file.Path == initAKSCloudFilepath {
			require.Equal(test, encodingGZIP, file.Encoding, file.Path)
			content = decodeCloudInitEncodedFile(test, base64.StdEncoding.EncodeToString(content))
		} else {
			require.Empty(test, file.Encoding, file.Path)
		}
		require.True(test, bytes.Equal(expected.contents, content), "content differs for %s", file.Path)
	}
	expectedBootCommand := "mkdir -p /opt/bin\n" +
		"for bin in aks-secure-tls-bootstrap-client ci-syslog-watcher.sh logrotate.sh; do\n" +
		"  [ -e /opt/bin/${bin} ] && continue\n" +
		"  [ -e /usr/local/bin/${bin} ] || continue\n" +
		"  ln -s /usr/local/bin/${bin} /opt/bin/\n" +
		"done\n"
	require.Equal(test, []string{expectedBootCommand}, decoded.BootCommands)
	expectedFiles[ignitionBootcmdScriptPath] = ignitionTarEntry{
		path: ignitionBootcmdScriptPath, mode: 0o755, owner: "root", contents: []byte("#!/bin/sh\n" + expectedBootCommand),
	}
	entries, err := buildIgnitionTarEntries(decoded)
	require.NoError(test, err)
	require.Len(test, entries, len(expectedFiles))
	for _, entry := range entries {
		require.Equal(test, expectedFiles[entry.path], entry, entry.path)
	}
	payload := generator.getLinuxNodeBootstrappingPayload(config)
	if config.IsFlatcar() || config.IsACL() {
		assertCloudInitIgnitionFiles(test, payload, expectedFiles)
		return
	}
	require.Equal(test, document, string(decodeCloudInitEncodedFile(test, payload)))
	if !config.ContainerService.IsAKSCustomCloud() {
		require.Less(test, len(payload), MaxCustomDataLength)
	}
	test.Logf("traditional custom data: %d encoded characters", len(payload))
}

func expectedCloudInitTraditionalFiles(test *testing.T, variables paramsMap, distroSuffix string, includeStart, customCloud bool) map[string]ignitionTarEntry {
	test.Helper()
	files := []struct {
		path     string
		variable string
		mode     int64
	}{
		{cseHelpersScriptFilepath, "provisionSource", 0o744},
		{cseHelpersScriptDistroFilepath, "provisionSource" + distroSuffix, 0o744},
		{"/opt/azure/containers/provision.sh", "provisionScript", 0o744},
		{cseInstallScriptFilepath, "provisionInstalls", 0o744},
		{cseInstallScriptDistroFilepath, "provisionInstalls" + distroSuffix, 0o744},
		{cseConfigScriptFilepath, "provisionConfigs", 0o744},
		{cseConfigGPUScriptFilepath, "provisionConfigsGPU", 0o744},
		{cseConfigLocalDNSScriptFilepath, "provisionConfigsLocalDNS", 0o744},
		{cseConfigKubeletScriptFilepath, "provisionConfigsKubelet", 0o744},
		{cseConfigNetworkScriptFilepath, "provisionConfigsNetwork", 0o744},
		{cseConfigAddonsScriptFilepath, "provisionConfigsAddons", 0o744},
		{cseConfigChronyScriptFilepath, "provisionConfigsChrony", 0o744},
		{"/etc/systemd/system/reconcile-private-hosts.service", "reconcilePrivateHostsService", 0o644},
		{"/etc/systemd/system/kubelet.service", "kubeletSystemdService", 0o600},
		{"/opt/azure-network/configure-azure-network.sh", "configureAzureNetworkScript", 0o755},
		{"/etc/udev/rules.d/99-azure-network.rules", "azureNetworkUdevRule", 0o644},
	}
	expected := make(map[string]ignitionTarEntry, len(files)+2)
	for _, file := range files {
		encoded, ok := variables[file.variable].(string)
		require.True(test, ok, file.variable)
		expected[file.path] = ignitionTarEntry{path: file.path, mode: file.mode, owner: "root", contents: decodeCloudInitEncodedFile(test, encoded)}
	}
	for path, variable := range map[string]string{
		"/opt/azure/containers/provision_start.sh": "provisionStartScript",
		initAKSCloudFilepath:                       "initAKSCloud",
	} {
		if (variable == "provisionStartScript" && !includeStart) || (variable == "initAKSCloud" && !customCloud) {
			continue
		}
		encoded, ok := variables[variable].(string)
		require.True(test, ok, variable)
		expected[path] = ignitionTarEntry{path: path, mode: 0o744, owner: "root", contents: decodeCloudInitEncodedFile(test, encoded)}
	}
	return expected
}

func decodeCloudInitEncodedFile(test *testing.T, encoded string) []byte {
	test.Helper()
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(test, err)
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	require.NoError(test, err)
	content, err := io.ReadAll(reader)
	require.NoError(test, err)
	require.NoError(test, reader.Close())
	return content
}

func assertCloudInitIgnitionFiles(test *testing.T, payload string, expected map[string]ignitionTarEntry) {
	test.Helper()
	var ignition struct {
		Storage struct {
			Files []struct {
				Path     string
				Mode     int
				Contents struct {
					Source      string
					Compression string
				}
			}
		}
	}
	document, err := base64.StdEncoding.DecodeString(payload)
	require.NoError(test, err)
	require.NoError(test, json.Unmarshal(document, &ignition))
	for _, file := range ignition.Storage.Files {
		if file.Path != ignitionFilesTarPath {
			continue
		}
		require.Equal(test, 0o600, file.Mode)
		require.Equal(test, encodingGZIP, file.Contents.Compression)
		encoded, found := strings.CutPrefix(file.Contents.Source, "data:;base64,")
		require.True(test, found)
		contents := decodeCloudInitEncodedFile(test, encoded)
		entries, err := decodeTarFiles(contents)
		require.NoError(test, err)
		require.Len(test, entries, len(expected))
		for _, entry := range entries {
			require.Equal(test, expected[entry.path].mode, entry.mode, entry.path)
			require.True(test, bytes.Equal(expected[entry.path].contents, []byte(entry.value)), "tar content differs for %s", entry.path)
		}
		return
	}
	test.Fatal("Ignition file archive is missing")
}

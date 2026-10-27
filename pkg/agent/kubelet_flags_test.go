package agent

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestKubeletOmissionsAfterPayloadEncoding(test *testing.T) {
	artifacts := filepath.Join(repoRoot(), "parts", "linux", "cloud-init", "artifacts")
	script := cseRoundTrip(test, filepath.Join(artifacts, "cse_config_kubelet.sh"))
	rawService, err := os.ReadFile(filepath.Join(artifacts, "kubelet.service"))
	require.NoError(test, err)
	for name, service := range map[string][]byte{
		"baked":   rawService,
		"payload": cseRoundTrip(test, filepath.Join(artifacts, "kubelet.service")),
	} {
		test.Run(name, func(test *testing.T) {
			root := test.TempDir()
			dropIns := filepath.Join(root, "etc/systemd/system/kubelet.service.d")
			require.NoError(test, os.MkdirAll(dropIns, 0700))
			require.NoError(test, os.MkdirAll(filepath.Join(root, "etc/default"), 0700))
			files := map[string][]byte{
				"renderer.sh":                        script,
				"etc/systemd/system/kubelet.service": service,
				"etc/default/kubelet":                []byte("KUBELET_FLAGS=--node-ip=10.0.0.4\n"),
				"etc/default/kubeletconfig.json":     []byte(`{"enableServer":true}`),
			}
			for path, content := range files {
				require.NoError(test, os.WriteFile(filepath.Join(root, path), content, 0600))
			}
			require.NoError(test, os.WriteFile(filepath.Join(dropIns, "10-componentconfig.conf"),
				[]byte("[Service]\nEnvironment=\"KUBELET_CONFIG_FILE_FLAGS=--config /etc/default/kubeletconfig.json\"\n"), 0600))
			runtimeDropIn := "[Service]\nEnvironment=\"KUBELET_CONTAINERD_FLAGS=--runtime-request-timeout=15m " +
				"--container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=/system.slice/containerd.service\"\n"
			require.NoError(test, os.WriteFile(filepath.Join(dropIns, "10-containerd-base-flag.conf"), []byte(runtimeDropIn), 0600))
			command := exec.Command("/bin/bash", "-c", `source "$1/renderer.sh"
reconcileKubeletConfigFlags /system.slice/containerd.service "$1"
cat "$1/etc/systemd/system/kubelet.service.d/11-kubelet-config-flags.conf"
KUBELET_FLAGS_TO_OMIT= reconcileKubeletConfigFlags /system.slice/containerd.service "$1"
test ! -e "$1/etc/systemd/system/kubelet.service.d/11-kubelet-config-flags.conf"`, "renderer-test", root)
			command.Env = append(os.Environ(), "KUBELET_CONFIG_FILE_ENABLED=true",
				"KUBELET_FLAGS_TO_OMIT="+base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server"]`)), "KUBELET_FLAGS=")
			output, err := command.CombinedOutput()
			require.NoError(test, err, "%s", output)
			require.Contains(test, string(output), "#AKS kubelet flag omissions v1 sha256:")
			require.Contains(test, string(output), "ExecStart=/opt/bin/kubelet")
			require.NotContains(test, string(output), "--enable-server")
		})
	}
}

func TestKubeletFlagsToOmitLegacyCSETransport(test *testing.T) {
	encoded := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server","--runtime-request-timeout"]`))
	for _, version := range []string{"1.31.0", "1.37.99", "1.38.0-beta.0", "1.38.0", "1.39.0"} {
		for _, configEnabled := range []bool{false, true} {
			for _, scriptlessCSE := range []bool{false, true} {
				test.Run(fmt.Sprintf("%s/config=%t/scriptless=%t", version, configEnabled, scriptlessCSE), func(test *testing.T) {
					config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
					config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = version
					config.EnableKubeletConfigFile = configEnabled
					config.EnableScriptlessCSECmd = scriptlessCSE
					generator := InitializeTemplateGenerator()
					legacy := generator.getLinuxNodeCSECommand(config)
					require.NotContains(test, legacy, "KUBELET_FLAGS_TO_OMIT=")
					for _, value := range []string{"", "W10=", "invalid!", strings.Repeat("a", 1025)} {
						config.EnabledFeatures = map[string]string{"KUBELET_FLAGS_TO_OMIT": value}
						require.Equal(test, legacy, generator.getLinuxNodeCSECommand(config))
					}
					config.EnabledFeatures = map[string]string{"KUBELET_FLAGS_TO_OMIT": encoded, "NOT_A_CSE_FEATURE": "true"}
					command := generator.getLinuxNodeCSECommand(config)
					require.Equal(test, legacy, strings.Replace(command, `KUBELET_FLAGS_TO_OMIT="`+encoded+`" `, "", 1))
					require.NotContains(test, command, "NOT_A_CSE_FEATURE=")
					start := strings.Index(command, "KUBELET_FLAGS_TO_OMIT=")
					end := strings.Index(command, "/usr/bin/nohup")
					require.Greater(test, end, start)
					require.GreaterOrEqual(test, start, 0)
					child := exec.Command("/bin/bash", "-c", command[start:end]+`/bin/bash -c 'printf "%s" "$KUBELET_FLAGS_TO_OMIT"'`)
					output, err := child.CombinedOutput()
					require.NoError(test, err, "%s", output)
					require.Equal(test, encoded, string(output))
				})
			}
		}
	}
}

func TestKubeletFlagsToOmitScriptlessPayloads(test *testing.T) {
	encodedOmissions := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server","--runtime-request-timeout"]`))
	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSFlatcarGen2,
		datamodel.AKSACLGen2TL,
	} {
		for _, mode := range []string{"nbc", "compare", "aksnodeconfig-only"} {
			for _, cseDelivery := range []bool{false, true} {
				test.Run(fmt.Sprintf("%s/%s/cse=%t", distro, mode, cseDelivery), func(test *testing.T) {
					config := newNodeCustomDataRenderConfig(distro)
					config.EnableScriptlessNBCCSECmd = mode != "aksnodeconfig-only"
					config.EnableScriptlessAKSNodeConfig = mode == "aksnodeconfig-only"
					config.ScriptlessCSEProvisionMode = cseDelivery
					if mode != "nbc" {
						config.AKSNodeConfigJSON = `{"version":"v1"}`
					}
					generator := InitializeTemplateGenerator()
					legacyPayload := generator.getLinuxNodeBootstrappingPayload(config)
					config.EnabledFeatures = map[string]string{}
					require.Equal(test, legacyPayload, generator.getLinuxNodeBootstrappingPayload(config))
					config.EnabledFeatures = map[string]string{"KUBELET_FLAGS_TO_OMIT": encodedOmissions}
					payload, err := base64.StdEncoding.DecodeString(generator.getLinuxNodeBootstrappingPayload(config))
					require.NoError(test, err)
					files := decodeKubeletScriptlessFiles(test, payload)
					require.Equal(test, "KUBELET_FLAGS_TO_OMIT="+encodedOmissions+"\n", string(files[enabledFeaturesFilepath]))
					featurePath := filepath.Join(test.TempDir(), "enabled_features.sh")
					require.NoError(test, os.WriteFile(featurePath, files[enabledFeaturesFilepath], 0600))
					child := exec.Command("/bin/bash", "-c", `source "$1"
anc_hotfix_log() { :; }
unset KUBELET_FLAGS_TO_OMIT
anc_hotfix_read_feature_flags "$2"
/bin/bash -c 'printf "%s" "$KUBELET_FLAGS_TO_OMIT"'`, "launcher-test",
						filepath.Join(repoRoot(), "parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.sh"), featurePath)
					output, err := child.CombinedOutput()
					require.NoError(test, err, "%s", output)
					require.Equal(test, encodedOmissions, string(output))
					assertScriptlessKubeletInputs(test, config, generator, files, encodedOmissions)
				})
			}
		}
	}
}

func assertScriptlessKubeletInputs(test *testing.T, config *datamodel.NodeBootstrappingConfiguration,
	generator *TemplateGenerator, files map[string][]byte, encodedOmissions string,
) {
	test.Helper()
	if config.ScriptlessCSEProvisionMode {
		require.NotContains(test, files, aksNodeConfigFilepath)
		require.NotContains(test, files, aksNbcCmdFilepath)
		prefix, suffix, found := strings.Cut(cseScriptlessPhase2Template, "%s")
		require.True(test, found)
		command := generator.getNodeBootstrappingCmd(config)
		require.True(test, strings.HasPrefix(command, prefix))
		require.True(test, strings.HasSuffix(command, suffix))
		encodedCommand := strings.TrimSuffix(strings.TrimPrefix(command, prefix), suffix)
		files = decodeKubeletScriptlessFiles(test, decodeCloudInitEncodedFile(test, encodedCommand))
	}
	if config.EnableScriptlessAKSNodeConfig {
		require.NotContains(test, files, aksNbcCmdFilepath)
	} else {
		require.Contains(test, string(files[aksNbcCmdFilepath]), `KUBELET_FLAGS_TO_OMIT="`+encodedOmissions+`" `)
	}
	if config.AKSNodeConfigJSON == "" {
		require.NotContains(test, files, aksNodeConfigFilepath)
	} else {
		require.Equal(test, config.AKSNodeConfigJSON, string(files[aksNodeConfigFilepath]))
		require.NotContains(test, string(files[aksNodeConfigFilepath]), "enabled_features")
	}
}

func decodeKubeletScriptlessFiles(test *testing.T, payload []byte) map[string][]byte {
	test.Helper()
	files := map[string][]byte{}
	if json.Valid(payload) {
		var ignition struct {
			Storage struct {
				Files []struct {
					Path     string
					Mode     int
					Contents struct{ Source, Compression string }
				}
			}
		}
		require.NoError(test, json.Unmarshal(payload, &ignition))
		for _, file := range ignition.Storage.Files {
			require.Equal(test, 0600, file.Mode)
			require.Equal(test, "gzip", file.Contents.Compression)
			require.True(test, strings.HasPrefix(file.Contents.Source, "data:;base64,"))
			require.NotContains(test, files, file.Path)
			files[file.Path] = decodeCloudInitEncodedFile(test, strings.TrimPrefix(file.Contents.Source, "data:;base64,"))
		}
	} else {
		fileBlock := regexp.MustCompile(`cat <<'EOF' \| base64 -d \| gzip -d >([^\n]+)\n([^\n]+)\nEOF\nchmod 0600 ([^\n]+)`)
		for _, matches := range fileBlock.FindAllSubmatch(payload, -1) {
			filePath := string(matches[1])
			require.Equal(test, filePath, string(matches[3]))
			require.NotContains(test, files, filePath)
			files[filePath] = decodeCloudInitEncodedFile(test, string(matches[2]))
		}
	}
	return files
}

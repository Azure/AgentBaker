package agent

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestKubeletConfigFlagsTraditionalPayload(test *testing.T) {
	for _, distro := range []datamodel.Distro{datamodel.AKSUbuntuContainerd2204Gen2, datamodel.AKSAzureLinuxV2Gen2, datamodel.AKSAzureLinuxV3Gen2} {
		test.Run(string(distro), func(test *testing.T) {
			config := newNodeCustomDataRenderConfig(distro)
			generator := InitializeTemplateGenerator()
			legacyCommand := generator.getLinuxNodeCSECommand(config)
			config.EnabledFeatures = map[string]string{"KUBELET_FLAGS_TO_OMIT": "WyItLWVuYWJsZS1zZXJ2ZXIiXQ"}
			require.Equal(test, legacyCommand, generator.getLinuxNodeCSECommand(config))
			payload := getCustomDataFromJSON(generator.getLinuxNodeCustomDataJSONObject(config))
			payloadSize := len(getBase64EncodedGzippedCustomScriptFromStr(payload))
			test.Logf("traditional payload: %d encoded characters", payloadSize)
			require.Less(test, payloadSize, MaxCustomDataLength)
			var cloudConfig cloudInit
			require.NoError(test, yaml.Unmarshal([]byte(payload), &cloudConfig))
			for _, file := range cloudConfig.WriteFiles {
				require.Equal(test, "gzip", file.Encoding, file.Path)
			}
			entries, err := buildIgnitionTarEntries(cloudConfig)
			require.NoError(test, err)
			foundHook := false
			for _, entry := range entries {
				require.NotEqual(test, "/opt/azure/containers/kubelet-flags.sh", entry.path)
				require.NotContains(test, string(entry.contents), "renderKubeletConfigFlags()")
				if entry.path == cseConfigKubeletScriptFilepath {
					foundHook = true
					require.Contains(test, string(entry.contents), "source /opt/azure/containers/kubelet-flags.sh")
					require.Equal(test, int64(0744), entry.mode)
				}
			}
			require.True(test, foundHook)
		})
	}
}

func TestKubeletConfigFlagsFinalHookRollback(test *testing.T) {
	artifacts := filepath.Join(repoRoot(), "parts/linux/cloud-init/artifacts")
	bakedScript, err := os.ReadFile(filepath.Join(artifacts, "cse_config_kubelet.sh"))
	require.NoError(test, err)
	helper, err := os.ReadFile(filepath.Join(artifacts, "kubelet-flags.sh"))
	require.NoError(test, err)
	for name, script := range map[string][]byte{
		"VHD": bakedScript, "NBC payload": cseRoundTrip(test, filepath.Join(artifacts, "cse_config_kubelet.sh")),
	} {
		test.Run(name, func(test *testing.T) {
			root := test.TempDir()
			dropInDirectory := filepath.Join(root, "etc/systemd/system/kubelet.service.d")
			helperPath := filepath.Join(root, "opt/azure/containers/kubelet-flags.sh")
			managedPath := filepath.Join(dropInDirectory, "11-kubelet-config-flags.conf")
			service := cseRoundTrip(test, filepath.Join(artifacts, "kubelet.service"))
			files := map[string][]byte{
				"etc/systemd/system/kubelet.service":                           service,
				"etc/default/kubelet":                                          []byte("KUBELET_FLAGS=--runtime-request-timeout=0s\n"),
				"etc/default/kubeletconfig.json":                               []byte(`{"enableServer":true}`),
				"etc/systemd/system/kubelet.service.d/10-componentconfig.conf": []byte("[Service]\nEnvironment=\"KUBELET_CONFIG_FILE_FLAGS=--config /etc/default/kubeletconfig.json\"\n"),
				"etc/systemd/system/kubelet.service.d/10-containerd-base-flag.conf": []byte("[Service]\nEnvironment=\"KUBELET_CONTAINERD_FLAGS=--runtime-request-timeout=15m " +
					"--container-runtime-endpoint=unix:///run/containerd/containerd.sock --runtime-cgroups=/system.slice/containerd.service\"\n"),
			}
			for path, content := range files {
				absolutePath := filepath.Join(root, path)
				require.NoError(test, os.MkdirAll(filepath.Dir(absolutePath), 0700))
				require.NoError(test, os.WriteFile(absolutePath, content, 0600))
			}
			hookStart := strings.Index(string(script), "    if [ -f /opt/azure/containers/kubelet-flags.sh ]; then")
			require.GreaterOrEqual(test, hookStart, 0)
			hookEnd := strings.Index(string(script[hookStart:]), "    if ! systemctlEnableAndStartNoBlock kubelet")
			require.Greater(test, hookEnd, 0)
			hook := string(script[hookStart : hookStart+hookEnd])
			hook = strings.ReplaceAll(hook, "/opt/azure/containers/", root+"/opt/azure/containers/")
			hook = strings.ReplaceAll(hook, `--apply "$runtime_cgroups"`, `--apply "$runtime_cgroups" "$TEST_ROOT"`)
			instruction := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server"]`))
			runHook := func(request, configEnabled, reloadResult string, expectedExitCode int) {
				command := exec.Command("/bin/bash", "-c", "systemctl() { return \"$RELOAD_RESULT\"; }\n"+hook)
				command.Env = append(os.Environ(), "TEST_ROOT="+root, "KUBELET_FLAGS_TO_OMIT="+request,
					"KUBELET_CONFIG_FILE_ENABLED="+configEnabled, "KUBELET_FLAGS=--runtime-request-timeout=0s",
					"runtime_cgroups=/system.slice/containerd.service", "ERR_KUBELET_START_FAIL=212", "RELOAD_RESULT="+reloadResult)
				output, err := command.CombinedOutput()
				if expectedExitCode == 0 {
					require.NoError(test, err, "%s", output)
				} else {
					require.Error(test, err, "%s", output)
				}
				require.Equal(test, expectedExitCode, command.ProcessState.ExitCode(), "%s", output)
			}
			runHook(instruction, "true", "0", 0)
			require.NoFileExists(test, managedPath)
			require.NoError(test, os.MkdirAll(filepath.Dir(helperPath), 0700))
			require.NoError(test, os.WriteFile(helperPath, helper, 0744))
			for _, rollback := range []struct{ request, configEnabled string }{{"", "true"}, {instruction, "false"}} {
				runHook(instruction, "true", "0", 0)
				managed, err := os.ReadFile(managedPath)
				require.NoError(test, err)
				require.Contains(test, string(managed), "#AKS kubelet flag omissions v1 sha256:")
				require.NotContains(test, string(managed), "--enable-server")
				runHook(rollback.request, rollback.configEnabled, "0", 0)
				require.NoFileExists(test, managedPath)
			}
			runHook("", "true", "1", 212)
			require.NoError(test, os.WriteFile(helperPath, []byte("return 9\n"), 0744))
			runHook(instruction, "true", "0", 212)
			for path, expected := range files {
				actual, err := os.ReadFile(filepath.Join(root, path))
				require.NoError(test, err)
				require.Equal(test, expected, actual)
			}
		})
	}
}

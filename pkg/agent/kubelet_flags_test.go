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
			test.Run(version+"/config="+map[bool]string{false: "off", true: "on"}[configEnabled], func(test *testing.T) {
				config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
				config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion = version
				config.EnableKubeletConfigFile = configEnabled
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

func TestKubeletFlagsToOmitScriptlessNBCTransport(test *testing.T) {
	encoded := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server"]`))
	config := newNodeCustomDataRenderConfig(datamodel.AKSUbuntuContainerd2204Gen2)
	config.EnableScriptlessNBCCSECmd = true
	config.EnabledFeatures = map[string]string{"KUBELET_FLAGS_TO_OMIT": encoded}
	generator := InitializeTemplateGenerator()
	boothook, err := base64.StdEncoding.DecodeString(generator.getScriptlessBoothook(config))
	require.NoError(test, err)
	require.Contains(test, string(boothook), getBase64EncodedGzippedCustomScriptFromStr("KUBELET_FLAGS_TO_OMIT="+encoded+"\n"))
	require.Contains(test, generator.getScriptlessNBCCmd(config), getBase64EncodedGzippedCustomScriptFromStr(generator.getLinuxNodeCSECommand(config)))
}

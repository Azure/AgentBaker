package scenario

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/parser"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestKataContainerdConfigScriptfulScriptlessParity(t *testing.T) {
	const containerdVersion = "2.3.4"

	nbc, err := baseTemplateLinux("eastus", "1.34.0", "amd64")
	require.NoError(t, err)
	nbc.ContainerdVersion = containerdVersion
	nbc.AgentPoolProfile.Distro = datamodel.AKSAzureLinuxV3Gen2Kata
	nbc.AgentPoolProfile.KubernetesConfig.ContainerdVersion = containerdVersion
	nbc.ContainerService.Properties.AgentPoolProfiles[0].Distro = datamodel.AKSAzureLinuxV3Gen2Kata
	nbc.ContainerService.Properties.AgentPoolProfiles[0].KubernetesConfig.ContainerdVersion = containerdVersion

	agentBaker, err := agent.NewAgentBaker()
	require.NoError(t, err)
	bootstrapping, err := agentBaker.GetNodeBootstrapping(context.Background(), nbc)
	require.NoError(t, err)
	scriptfulConfig := decodeContainerdConfigFromCSE(t, bootstrapping.CSE)

	nodeConfig, err := nbcToAKSNodeConfigV1(nbc)
	require.NoError(t, err)
	t.Setenv("PATH", t.TempDir())
	cmd, err := parser.BuildCSECmd(context.Background(), nodeConfig, nil)
	require.NoError(t, err)
	scriptlessConfig := decodeContainerdConfigFromEnv(t, cmd.Env)

	for _, rendered := range []string{scriptfulConfig, scriptlessConfig} {
		require.True(t, strings.HasPrefix(rendered, "version = 4\n"), rendered)
		require.NotContains(t, rendered, `io.containerd.grpc.v1.cri`)
		require.Contains(t, rendered, `plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata]`)
		require.Contains(t, rendered, `plugins."io.containerd.cri.v1.runtime".containerd.runtimes.kata-v2]`)
	}

	containerdBinary := os.Getenv("CONTAINERD_TEST_BINARY")
	if containerdBinary == "" {
		t.Log("CONTAINERD_TEST_BINARY is not set; skipping real containerd validation")
		return
	}
	validateRenderedContainerdConfig(t, containerdBinary, "scriptful", scriptfulConfig)
	validateRenderedContainerdConfig(t, containerdBinary, "scriptless", scriptlessConfig)
}

func decodeContainerdConfigFromCSE(t *testing.T, cse string) string {
	t.Helper()
	match := regexp.MustCompile(`CONTAINERD_CONFIG_CONTENT="([^"]+)"`).FindStringSubmatch(cse)
	require.Len(t, match, 2, cse)
	return decodeContainerdConfig(t, match[1])
}

func decodeContainerdConfigFromEnv(t *testing.T, env []string) string {
	t.Helper()
	for _, entry := range env {
		if encoded, found := strings.CutPrefix(entry, "CONTAINERD_CONFIG_CONTENT="); found {
			return decodeContainerdConfig(t, encoded)
		}
	}
	t.Fatal("CONTAINERD_CONFIG_CONTENT is missing from the CSE environment")
	return ""
}

func decodeContainerdConfig(t *testing.T, encoded string) string {
	t.Helper()
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	return string(decoded)
}

func validateRenderedContainerdConfig(t *testing.T, binary, name, rendered string) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), name+"-containerd.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(rendered), 0o600))

	cmd := exec.Command(binary, "--config", configPath, "config", "dump")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(), stderr.String())
	require.Empty(t, stderr.String())
	require.True(t, strings.HasPrefix(stdout.String(), "version = 4\n"), stdout.String())
}

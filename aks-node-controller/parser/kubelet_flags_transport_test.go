package parser

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/stretchr/testify/require"
)

func TestKubeletFlagsToOmitInheritedByCSEChild(test *testing.T) {
	encoded := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server","--runtime-request-timeout"]`))
	featuresPath := filepath.Join(test.TempDir(), "enabled_features.sh")
	require.NoError(test, os.WriteFile(featuresPath, []byte("KUBELET_FLAGS_TO_OMIT="+encoded+"\n"), 0600))
	launcherScript := `source ../../parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.sh
anc_hotfix_log() { :; }
anc_hotfix_read_feature_flags "$1"
/bin/bash -c 'printf "%s" "$KUBELET_FLAGS_TO_OMIT"'`
	launcher := exec.Command("/bin/bash", "-c", launcherScript, "launcher-test", featuresPath)
	output, err := launcher.CombinedOutput()
	require.NoError(test, err, "%s", output)
	require.Equal(test, encoded, string(output))
	test.Setenv("KUBELET_FLAGS_TO_OMIT", string(output))
	command, err := BuildCSECmd(context.Background(), &aksnodeconfigv1.Configuration{}, nil)
	require.NoError(test, err)
	child := exec.Command("/bin/bash", "-c", `printf '%s' "$KUBELET_FLAGS_TO_OMIT"`)
	child.Env = command.Env
	output, err = child.CombinedOutput()
	require.NoError(test, err)
	require.Equal(test, encoded, string(output))
}

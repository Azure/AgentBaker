package parser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
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

func TestKubeletFlagsToOmitFromCustomDataToCSEChild(test *testing.T) {
	encodedOmissions := base64.RawStdEncoding.EncodeToString([]byte(`["--enable-server","--runtime-request-timeout"]`))
	for _, mode := range []string{"cloud-init", "custom-data-only", "flatcar"} {
		for _, instruction := range []struct {
			name      string
			value     string
			inherited string
			features  map[string]string
		}{
			{name: "absent"},
			{name: "empty", inherited: encodedOmissions, features: map[string]string{"KUBELET_FLAGS_TO_OMIT": ""}},
			{name: "present", value: encodedOmissions, features: map[string]string{"KUBELET_FLAGS_TO_OMIT": encodedOmissions}},
		} {
			test.Run(mode+"/"+instruction.name, func(test *testing.T) {
				test.Setenv("KUBELET_FLAGS_TO_OMIT", instruction.inherited)
				config := &aksnodeconfigv1.Configuration{
					CustomDataOnlyProvisioning: mode == "custom-data-only",
					EnabledFeatures:            instruction.features,
				}
				expectedOmissions := instruction.value
				buildCustomData := nodeconfigutils.CustomData
				if mode == "flatcar" {
					buildCustomData = nodeconfigutils.CustomDataFlatcar
				}
				customData, err := buildCustomData(config)
				require.NoError(test, err)
				payload, err := base64.StdEncoding.DecodeString(customData)
				require.NoError(test, err)
				featureContent := decodeKubeletFeaturesFromCustomData(test, payload, mode == "flatcar")
				featuresPath := filepath.Join(test.TempDir(), "enabled_features.sh")
				if instruction.name == "absent" {
					require.Nil(test, featureContent)
				} else {
					require.Equal(test, "KUBELET_FLAGS_TO_OMIT="+expectedOmissions+"\n", string(featureContent))
					require.NoError(test, os.WriteFile(featuresPath, featureContent, 0600))
				}
				launcher := exec.Command("/bin/bash", "-c", `source ../../parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.sh
anc_hotfix_log() { :; }
anc_hotfix_read_feature_flags "$1"
/bin/bash -c 'printf "%s" "${KUBELET_FLAGS_TO_OMIT:-}"'`, "launcher-test", featuresPath)
				output, err := launcher.CombinedOutput()
				require.NoError(test, err, "%s", output)
				require.Equal(test, expectedOmissions, string(output))
				test.Setenv("KUBELET_FLAGS_TO_OMIT", string(output))
				command, err := BuildCSECmd(context.Background(), config, nil)
				require.NoError(test, err)
				child := exec.Command("/bin/bash", "-c", `printf '%s' "${KUBELET_FLAGS_TO_OMIT:-}"`)
				child.Env = command.Env
				output, err = child.CombinedOutput()
				require.NoError(test, err, "%s", output)
				require.Equal(test, expectedOmissions, string(output))
			})
		}
	}
}

func decodeKubeletFeaturesFromCustomData(test *testing.T, payload []byte, flatcar bool) []byte {
	test.Helper()
	if flatcar {
		var ignition struct {
			Storage struct {
				Files []struct {
					Path     string
					Contents struct{ Source string }
				}
			}
		}
		require.NoError(test, json.Unmarshal(payload, &ignition))
		for _, file := range ignition.Storage.Files {
			if file.Path == nodeconfigutils.EnabledFeaturesFilePath {
				featureContent, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(file.Contents.Source, "data:;base64,"))
				require.NoError(test, err)
				return featureContent
			}
		}
		return nil
	}
	featureBlock := regexp.MustCompile(`(?s)cat <<'EOF' >` + regexp.QuoteMeta(nodeconfigutils.EnabledFeaturesFilePath) + `\n(.*?)EOF\n`)
	if matches := featureBlock.FindSubmatch(payload); matches != nil {
		return matches[1]
	}
	return nil
}

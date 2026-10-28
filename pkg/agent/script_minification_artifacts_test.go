package agent

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/Azure/agentbaker/parts"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestCSEScriptMinificationArtifacts(t *testing.T) {
	outputDir := os.Getenv("CSE_SCRIPT_DIFF_DIR")
	if outputDir == "" {
		t.Skip("set CSE_SCRIPT_DIFF_DIR to export rendered and minified script diffs")
	}
	require.True(t, filepath.IsAbs(outputDir), "CSE_SCRIPT_DIFF_DIR must be an absolute path")
	scripts := discoverCSEScripts(t)
	require.NotEmpty(t, scripts)

	for _, distro := range []datamodel.Distro{
		datamodel.AKSUbuntuContainerd2204Gen2,
		datamodel.AKSUbuntuContainerd2404Gen2,
		datamodel.AKSUbuntuMinimalContainerd2604Gen2,
		datamodel.AKSCBLMarinerV2Gen2,
		datamodel.AKSAzureLinuxV2Gen2,
		datamodel.AKSAzureLinuxV3Gen2,
	} {
		for _, cloud := range []string{"public", "custom"} {
			t.Run(string(distro)+"/"+cloud, func(t *testing.T) {
				config := newNodeCustomDataRenderConfig(distro)
				config.EnableKubeletConfigFile = true
				config.AgentPoolProfile.LocalDNSProfile = &datamodel.LocalDNSProfile{EnableLocalDNS: true}
				if cloud == "custom" {
					config.ContainerService.Properties.CustomCloudEnv = &datamodel.CustomCloudEnv{Name: "akscustom"}
				}
				dir := filepath.Join(outputDir, string(distro), cloud)
				for _, script := range scripts {
					t.Run(script, func(t *testing.T) {
						// Mirror production preprocessing without using the shell parser/printer
						// on the baseline: otherwise normalization could hide minifier changes.
						raw, err := parts.Templates.ReadFile(script)
						require.NoError(t, err)
						templ, err := template.New("ContainerService template").
							Option("missingkey=error").
							Funcs(getContainerServiceFuncMap(config)).
							Parse(string(removeComments(raw)))
						require.NoError(t, err)
						var rendered bytes.Buffer
						require.NoError(t, templ.Execute(&rendered, config.ContainerService))
						before := []byte(strings.ReplaceAll(rendered.String(), "\r\n", "\n"))
						beforePath := filepath.Join("before", filepath.FromSlash(script))
						writeMinificationArtifact(t, filepath.Join(dir, beforePath), before)

						encoded := getBase64EncodedGzippedCustomScript(script, config)
						compressed, err := base64.StdEncoding.Strict().DecodeString(encoded)
						require.NoError(t, err)
						after, err := getGzipDecodedValue(compressed)
						require.NoError(t, err)
						afterPath := filepath.Join("after", filepath.FromSlash(script))
						writeMinificationArtifact(t, filepath.Join(dir, afterPath), after)

						cmd := exec.Command("git", "--no-pager", "diff", "--no-index",
							"--no-ext-diff", "--no-textconv", "--no-color", "--", beforePath, afterPath)
						cmd.Dir = dir
						diff, err := cmd.CombinedOutput()
						if err != nil {
							var exitErr *exec.ExitError
							require.True(t, errors.As(err, &exitErr), "git diff: %v\n%s", err, diff)
							require.Equal(t, 1, exitErr.ExitCode(), "git diff: %s", diff)
						}
						writeMinificationArtifact(t, filepath.Join(dir, "diff", filepath.FromSlash(script)+".diff"), diff)
						cseValidateBashSyntax(t, script, before)
						cseValidateBashSyntax(t, script, after)
					})
				}
			})
		}
	}
	t.Logf("script diffs written to %s", outputDir)
}

func writeMinificationArtifact(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, content, 0600))
}

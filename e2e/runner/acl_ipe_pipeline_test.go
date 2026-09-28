package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func readPipelineYAML(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, yaml.Unmarshal(data, &document))
	return document
}

func pipelineMap(t *testing.T, value any) map[string]any {
	t.Helper()
	result, ok := value.(map[string]any)
	require.True(t, ok, "expected YAML map, got %T", value)
	return result
}

func pipelineList(t *testing.T, value any) []any {
	t.Helper()
	result, ok := value.([]any)
	require.True(t, ok, "expected YAML list, got %T", value)
	return result
}

func namedPipelineItem(t *testing.T, items []any, key, name string) map[string]any {
	t.Helper()
	for _, item := range items {
		mapping := pipelineMap(t, item)
		if mapping[key] == name {
			return mapping
		}
	}
	t.Fatalf("missing YAML %s=%s", key, name)
	return nil
}

func TestReleasePipelineACLIPEModeWiring(t *testing.T) {
	release := readPipelineYAML(t, filepath.Join("..", "..", ".pipelines", ".vsts-vhd-builder-release.yaml"))
	template := readPipelineYAML(t, filepath.Join("..", "..", ".pipelines", "templates", "e2e-template.yaml"))

	modeParam := namedPipelineItem(t, pipelineList(t, release["parameters"]), "name", "aclIpeExpectedMode")
	require.Equal(t, "none", modeParam["default"])
	require.Equal(t, []any{"none", "off", "audit"}, pipelineList(t, modeParam["values"]))
	for name, expected := range map[string]string{
		"aclTlSourceGallery": "ae6ab47f-1fc9-4b67-8f82-4636119119c9-ACL",
		"aclTlSourceImage":   "acl-3.0-amd64",
		"aclTlSourceVersion": "20260909.1200337.0",
	} {
		require.Equal(t, expected, namedPipelineItem(t, pipelineList(t, release["parameters"]), "name", name)["default"])
	}
	templateMode := namedPipelineItem(t, pipelineList(t, template["parameters"]), "name", "aclIpeExpectedMode")
	require.Equal(t, modeParam["default"], templateMode["default"])
	require.Equal(t, modeParam["values"], templateMode["values"])

	build := namedPipelineItem(t, pipelineList(t, release["stages"]), "stage", "build")
	aclJob := namedPipelineItem(t, pipelineList(t, build["jobs"]), "job", "buildacltlgen2")
	setup := namedPipelineItem(t, pipelineList(t, aclJob["steps"]), "displayName", "Setup Build Variables")
	source := pipelineMap(t, setup["env"])
	for key, value := range map[string]string{
		"SIG_SOURCE_GALLERY_UNIQUE_NAME": "${{ parameters.aclTlSourceGallery }}",
		"SIG_SOURCE_IMAGE_NAME":          "${{ parameters.aclTlSourceImage }}",
		"SIG_SOURCE_IMAGE_VERSION":       "${{ parameters.aclTlSourceVersion }}",
	} {
		require.Equal(t, value, source[key])
	}
	script, ok := setup["bash"].(string)
	require.True(t, ok)
	for _, selector := range []string{"SIG_SOURCE_GALLERY_UNIQUE_NAME", "SIG_SOURCE_IMAGE_NAME", "SIG_SOURCE_IMAGE_VERSION"} {
		require.Contains(t, script, `[[ ! "$`+selector+`" =~`, "missing source must fail the existing setup guard")
	}
	require.Contains(t, script, "exit 1")

	e2e := namedPipelineItem(t, pipelineList(t, release["stages"]), "stage", "e2e")
	require.Equal(t, "and(succeeded(), or(ne('${{ parameters.aclIpeExpectedMode }}', 'none'), ne(variables.SKIP_E2E_TESTS, 'true')))", e2e["condition"])
	e2eJob := namedPipelineItem(t, pipelineList(t, e2e["jobs"]), "template", "./templates/e2e-template.yaml")
	jobParams := pipelineMap(t, e2eJob["parameters"])
	require.Equal(t, "${{ parameters.aclIpeExpectedMode }}", jobParams["aclIpeExpectedMode"])
	require.Equal(t, true, jobParams["IgnoreScenariosWithMissingVhd"])
	require.Equal(t, true, jobParams["useVhdMetadataArtifacts"])

	job := namedPipelineItem(t, pipelineList(t, template["jobs"]), "job", "e2e")
	require.Equal(t, "and(succeeded(), or(ne('${{ parameters.aclIpeExpectedMode }}', 'none'), ne(variables.SKIP_E2E_TESTS, 'true')))", job["condition"])
	task := namedPipelineItem(t, pipelineList(t, job["steps"]), "displayName", "Run AgentBaker E2E")
	require.Equal(t, "AzureCLI@2", task["task"])
	env := pipelineMap(t, task["env"])
	_, unconditional := env["ACL_IPE_EXPECTED_MODE"]
	require.False(t, unconditional, "ordinary E2E runs must retain their existing environment")
	conditional := pipelineMap(t, env["${{ if ne(parameters.aclIpeExpectedMode, 'none') }}"])
	require.Equal(t, "${{ parameters.aclIpeExpectedMode }}", conditional["ACL_IPE_EXPECTED_MODE"])
	for _, mode := range []string{"off", "audit"} {
		require.Equal(t, mode, strings.ReplaceAll(conditional["ACL_IPE_EXPECTED_MODE"].(string),
			"${{ parameters.aclIpeExpectedMode }}", mode))
	}

	scriptPath := filepath.Join("..", "..", ".pipelines", "scripts", "e2e_run.sh")
	scriptBody, err := os.ReadFile(scriptPath)
	require.NoError(t, err)
	normalizedScript := strings.ReplaceAll(string(scriptBody), "\r\n", "\n")
	require.Contains(t, normalizedScript,
		"(\n  unset ACL_IPE_EXPECTED_MODE\n  go test -count=1 ./...\n)\n",
		"unit tests must not inherit the scenario mode, but the runner must")
	require.Contains(t, normalizedScript,
		"scenario_selectors=()\nif [ -n \"${ACL_IPE_EXPECTED_MODE:-}\" ]; then\n  scenario_selectors=(ACL)\nfi\n",
		"only opted-in runs may add the exact ACL scenario selector")
	require.Contains(t, normalizedScript,
		"\n  --output grouped \\\n  \"${scenario_selectors[@]}\"",
		"the E2E runner must receive the opt-in ACL-only selector")
	require.Less(t, strings.Index(normalizedScript, "scenario_selectors=(ACL)"),
		strings.Index(normalizedScript, "\ngo run . run"),
		"the ACL-only selector must be set before invoking the runner")
}

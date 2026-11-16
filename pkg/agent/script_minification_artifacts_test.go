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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"mvdan.cc/sh/v3/syntax"
)

func TestCSEScriptMinificationArtifacts(t *testing.T) {
	outputDir := os.Getenv("CSE_SCRIPT_DIFF_DIR")
	if outputDir != "" {
		require.True(t, filepath.IsAbs(outputDir), "CSE_SCRIPT_DIFF_DIR must be an absolute path")
	}
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
						// Existing comment removal is part of the accepted baseline. Render
						// without the production minifier or encoder so their changes remain visible.
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
						encoded := getBase64EncodedGzippedCustomScript(script, config)
						compressed, err := base64.StdEncoding.Strict().DecodeString(encoded)
						require.NoError(t, err)
						after, err := getGzipDecodedValue(compressed)
						require.NoError(t, err)
						if outputDir != "" {
							writeMinificationDiff(t, dir, script, before, after)
						}
						require.Empty(t, shellCodeDiff(t, script, before, after),
							"production rendering changed shell code (-source +production)")
						cseValidateBashSyntax(t, script, before)
						cseValidateBashSyntax(t, script, after)
					})
				}
			})
		}
	}
	if outputDir != "" {
		t.Logf("script diffs written to %s", outputDir)
	}
}

func shellCodeDiff(t *testing.T, name string, before, after []byte) string {
	t.Helper()
	type shellCode struct {
		Shebang string
		File    *syntax.File
	}
	parse := func(script []byte) shellCode {
		t.Helper()
		// Discard comments, but retain literal content (including heredocs).
		file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(bytes.NewReader(script), name)
		require.NoError(t, err)
		code := shellCode{File: file}
		if bytes.HasPrefix(script, []byte("#!")) {
			code.Shebang, _, _ = strings.Cut(string(script), "\n")
		}
		return code
	}
	return cmp.Diff(parse(before), parse(after),
		cmpopts.IgnoreTypes(syntax.Pos{}),
		// Minification may spell ${name} as $name; the parsed parameter and
		// all expansion operators must still match.
		cmpopts.IgnoreFields(syntax.ParamExp{}, "Short"),
	)
}

func TestShellCodeDiff(t *testing.T) {
	tests := []struct {
		name   string
		before string
		after  string
		equal  bool
	}{
		{
			name:   "comments whitespace and statement separators",
			before: "#!/bin/bash\n# comment\n\nif true; then\n    echo ok # trailing comment\nfi\n",
			after:  "#!/bin/bash\nif true;then echo ok;fi\n",
			equal:  true,
		},
		{
			name:   "short parameter expansion",
			before: `echo "${value}"` + "\n",
			after:  `echo "$value"` + "\n",
			equal:  true,
		},
		{
			name:   "preserved heredoc and multiline literal",
			before: "# comment\ncat <<'EOF'\n# literal\n  content\n\nEOF\n\n  echo 'first\n\nlast'\n",
			after:  "cat <<'EOF'\n# literal\n  content\n\nEOF\necho 'first\n\nlast'\n",
			equal:  true,
		},
		{
			name:   "changed parameter boundary",
			before: "echo ${value}suffix\n",
			after:  "echo $valuesuffix\n",
		},
		{
			name:   "changed positional parameter",
			before: "echo ${10}\n",
			after:  "echo $10\n",
		},
		{
			name:   "added command",
			before: "echo ok\n",
			after:  "echo ok; unexpected_command\n",
		},
		{
			name:   "removed command",
			before: "check_configuration\necho ok\n",
			after:  "echo ok\n",
		},
		{
			name:   "changed argument",
			before: "echo ok\n",
			after:  "echo unexpected\n",
		},
		{
			name:   "injected command substitution",
			before: `echo "ok"` + "\n",
			after:  `echo "$(unexpected_command)"` + "\n",
		},
		{
			name:   "injected process substitution",
			before: "cat input\n",
			after:  "cat <(unexpected_command)\n",
		},
		{
			name:   "changed control flow",
			before: "check_configuration && start_service\n",
			after:  "check_configuration; start_service\n",
		},
		{
			name:   "background command",
			before: "check_configuration; start_service\n",
			after:  "check_configuration & start_service\n",
		},
		{
			name:   "changed redirect",
			before: "echo ok > output\n",
			after:  "echo ok >> output\n",
		},
		{
			name:   "changed function body",
			before: "setup() { echo ok; }\n",
			after:  "setup() { echo ok; unexpected_command; }\n",
		},
		{
			name:   "removed quoting",
			before: `echo "$value"` + "\n",
			after:  "echo $value\n",
		},
		{
			name:   "changed whitespace inside literal",
			before: "echo 'a  b'\n",
			after:  "echo 'a b'\n",
		},
		{
			name:   "removed comment-like literal content",
			before: "echo 'first\n# literal\nlast'\n",
			after:  "echo 'first\nlast'\n",
		},
		{
			name:   "changed heredoc payload",
			before: "cat <<'EOF'\n# literal\n  content\n\nEOF\n",
			after:  "cat <<'EOF'\ncontent\nEOF\n",
		},
		{
			name:   "enabled heredoc expansion",
			before: "cat <<'EOF'\n$variable\nEOF\n",
			after:  "cat <<EOF\n$variable\nEOF\n",
		},
		{
			name:   "changed interpreter",
			before: "#!/bin/bash\necho ok\n",
			after:  "#!/bin/other\necho ok\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := shellCodeDiff(t, tt.name, []byte(tt.before), []byte(tt.after))
			if tt.equal {
				require.Empty(t, diff)
			} else {
				require.NotEmpty(t, diff, "unexpected shell code change was accepted")
			}
		})
	}
}

func writeMinificationDiff(t *testing.T, dir, script string, before, after []byte) {
	t.Helper()
	beforePath := filepath.Join("before", filepath.FromSlash(script))
	afterPath := filepath.Join("after", filepath.FromSlash(script))
	writeMinificationArtifact(t, filepath.Join(dir, beforePath), before)
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
}

func writeMinificationArtifact(t *testing.T, path string, content []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	require.NoError(t, os.WriteFile(path, content, 0600))
}

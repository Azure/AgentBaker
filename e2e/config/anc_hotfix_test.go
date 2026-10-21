package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

func TestResolveANCHotfixVersions(t *testing.T) {
	for _, tc := range []struct {
		target   string
		required bool
		base     string
		want     string
	}{
		{base: "202608.21.0", want: "202608.21.1"},
		{target: "202608.26.2", base: "202608.26.0", want: "202608.26.2"},
		{target: "202610.20.15", required: true, base: "202610.20.0", want: "202610.20.15"},
		{required: true},
		{target: "202608.26.0"},
		{target: "202608.26.-1"},
		{target: "202608.26.01"},
		{target: "202608.26"},
		{target: "v202608.26.2"},
		{target: "202608.26.2-ubuntu22.04u1"},
		{target: "202613.26.2"},
		{target: "202602.30.2"},
		{target: "000001.01.2"},
		{target: "202608.01.2", base: "202608.01.0", want: "202608.01.2"},
		{target: "202802.29.2", base: "202802.29.0", want: "202802.29.2"},
		{target: "202608.26.18446744073709551616"},
		{target: " 202608.26.2"},
		{target: "$(ANC_HOTFIX_TARGET_VERSION)"},
	} {
		t.Run(tc.target+map[bool]string{true: "/required", false: "/optional"}[tc.required], func(t *testing.T) {
			cfg := DefaultConfiguration()
			cfg.ANCHotfixTargetVersion = tc.target
			cfg.RequireANCHotfixValidation = tc.required
			err := cfg.ResolveANCHotfixVersions()
			if tc.want == "" {
				require.ErrorContains(t, err, "ANC_HOTFIX_TARGET_VERSION")
				return
			}
			require.NoError(t, err)
			require.Equal(t, ANCHotfixVersions{Base: tc.base, Target: tc.want}, cfg.ANCHotfixVersions)
		})
	}
}

func TestANCHotfixQueueVariablesAreNotOverridden(t *testing.T) {
	type pipeline struct {
		Variables  map[string]string
		Parameters []struct{ Name string }
		Jobs       []struct {
			Parameters map[string]string
			Steps      []struct {
				Env map[string]any
			}
		}
	}
	read := func(name string) pipeline {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", ".pipelines", name))
		require.NoError(t, err)
		var p pipeline
		require.NoError(t, yaml.Unmarshal(data, &p))
		return p
	}
	template := read(filepath.Join("templates", "e2e-template.yaml"))
	parameters := make(map[string]bool)
	for _, parameter := range template.Parameters {
		parameters[parameter.Name] = true
	}
	require.NotContains(t, parameters, "ancHotfixTargetVersion")
	require.NotContains(t, parameters, "requireANCHotfixValidation")
	for _, step := range template.Jobs[0].Steps {
		require.NotContains(t, step.Env, "ANC_HOTFIX_TARGET_VERSION")
		require.NotContains(t, step.Env, "REQUIRE_ANC_HOTFIX_VALIDATION")
	}
	for _, name := range []string{"e2e-tme.yaml", "e2e.yaml"} {
		p := read(name)
		require.NotContains(t, p.Variables, "ANC_HOTFIX_TARGET_VERSION")
		require.NotContains(t, p.Variables, "REQUIRE_ANC_HOTFIX_VALIDATION")
		require.NotContains(t, p.Jobs[0].Parameters, "ancHotfixTargetVersion")
		require.NotContains(t, p.Jobs[0].Parameters, "requireANCHotfixValidation")
	}
}

func TestANCHotfixFlagsResolveAfterParsing(t *testing.T) {
	saved := *Config
	t.Cleanup(func() { *Config = saved })
	t.Setenv("ANC_HOTFIX_TARGET_VERSION", "202608.26.2")
	t.Setenv("REQUIRE_ANC_HOTFIX_VALIDATION", "true")
	for _, tc := range []struct {
		name     string
		args     []string
		target   string
		required bool
	}{
		{name: "environment", target: "202608.26.2", required: true},
		{name: "flag override", args: []string{"--anc-hotfix-target-version", "202610.20.3"}, target: "202610.20.3", required: true},
		{name: "optional flag override", args: []string{"--require-anc-hotfix-validation=false"}, target: "202608.26.2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			*Config = *DefaultConfiguration()
			cmd := &cli.Command{
				Flags: Flags(),
				Action: func(context.Context, *cli.Command) error {
					require.Equal(t, tc.required, Config.RequireANCHotfixValidation)
					require.Equal(t, tc.target, Config.ANCHotfixTargetVersion)
					require.Equal(t, defaultANCHotfixVersions, Config.ANCHotfixVersions)
					require.NoError(t, Config.ResolveANCHotfixVersions())
					require.Equal(t, tc.target, Config.ANCHotfixVersions.Target)
					require.Equal(t, tc.target[:9]+".0", Config.ANCHotfixVersions.Base)
					return nil
				},
			}
			require.NoError(t, cmd.Run(context.Background(), append([]string{"e2e"}, tc.args...)))
		})
	}
}

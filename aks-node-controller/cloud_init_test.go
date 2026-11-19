package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudInitializationSettings(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	nbcPath := filepath.Join(dir, "nbc.sh")
	executed := filepath.Join(dir, "executed")
	require.NoError(t, os.WriteFile(configPath, []byte(`{
		"cluster_config":{"location":"ussec"},
		"custom_cloud_config":{"custom_cloud_env_name":"akscustom","repo_depot_endpoint":"https://depot/ubuntu"}
	}`), 0600))
	require.NoError(t, os.WriteFile(nbcPath, []byte(
		`LOCATION=usnat REPO_DEPOT_ENDPOINT='https://live-depot/ubuntu'; touch '`+executed+`'`), 0600))

	for _, tc := range []struct {
		name     string
		flags    ProvisionFlags
		location string
		endpoint string
	}{
		{"config", ProvisionFlags{ProvisionConfig: configPath}, "ussec", "https://depot/ubuntu"},
		{"NBC", ProvisionFlags{NBCCmd: nbcPath}, "usnat", "https://live-depot/ubuntu"},
		{"NBC takes precedence", ProvisionFlags{ProvisionConfig: "missing", NBCCmd: nbcPath},
			"usnat", "https://live-depot/ubuntu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings, err := cloudInitializationSettings(tc.flags)
			require.NoError(t, err)
			require.Equal(t, tc.location, settings["LOCATION"])
			require.Equal(t, tc.endpoint, settings["REPO_DEPOT_ENDPOINT"])
			require.NoFileExists(t, executed)
		})
	}

	for _, input := range []string{
		`{"cluster_config":{"location":"westus2"}}`,
		`{"cluster_config":{"location":"westus2"},"custom_cloud_config":{"repo_depot_endpoint":"ignored"}}`,
	} {
		require.NoError(t, os.WriteFile(configPath, []byte(input), 0600))
		settings, err := cloudInitializationSettings(ProvisionFlags{ProvisionConfig: configPath})
		require.NoError(t, err)
		require.Equal(t, "westus2", settings["LOCATION"])
		require.Empty(t, settings["REPO_DEPOT_ENDPOINT"])
	}

	for _, input := range []string{
		`{`,
		`{"cluster_config":{"location":" "}}`,
		`{"cluster_config":{"location":"ussec"},"kubelet_config":123}`,
	} {
		require.NoError(t, os.WriteFile(configPath, []byte(input), 0600))
		_, err := cloudInitializationSettings(ProvisionFlags{ProvisionConfig: configPath})
		require.Error(t, err)
	}
	require.NoError(t, os.WriteFile(nbcPath, []byte("REPO_DEPOT_ENDPOINT=https://depot"), 0600))
	for _, flags := range []ProvisionFlags{
		{}, {NBCCmd: nbcPath}, {NBCCmd: dir}, {NBCCmd: "missing"}, {ProvisionConfig: "missing"},
	} {
		_, err := cloudInitializationSettings(flags)
		require.Error(t, err)
	}
}

func TestInitializeCloud(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cloud initialization executes Linux bash")
	}
	dir := t.TempDir()
	nbcPath := filepath.Join(dir, "nbc.sh")
	scriptPath := filepath.Join(dir, "init.sh")
	outputPath := filepath.Join(dir, "settings")
	require.NoError(t, os.WriteFile(nbcPath, []byte("LOCATION=usnat REPO_DEPOT_ENDPOINT=https://live-depot"), 0600))
	t.Setenv("LOCATION", "baked-location")
	t.Setenv("REPO_DEPOT_ENDPOINT", "https://baked-depot")
	t.Setenv("AKS_CLOUD_INIT_SHA256", "inherited-handoff")
	t.Setenv("CLOUD_INIT_TEST_OUTPUT", outputPath)
	app := &App{cmdRun: cmdRunner}

	script := `printf '%s\n' "$#:$LOCATION:$REPO_DEPOT_ENDPOINT:$AKS_CLOUD_INIT_SHA256" > "$CLOUD_INIT_TEST_OUTPUT"`
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0600))
	hash, err := app.initializeCloud(t.Context(), ProvisionFlags{NBCCmd: nbcPath}, scriptPath)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte(script))), hash)
	settings, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "0:usnat:https://live-depot:\n", string(settings))

	require.NoError(t, os.WriteFile(scriptPath, []byte("exit 23\n"), 0600))
	hash, err = app.initializeCloud(t.Context(), ProvisionFlags{NBCCmd: nbcPath}, scriptPath)
	require.ErrorContains(t, err, "exit status 23")
	require.Empty(t, hash)

	hash, err = app.initializeCloud(t.Context(), ProvisionFlags{NBCCmd: nbcPath}, filepath.Join(dir, "missing"))
	require.ErrorContains(t, err, "read cloud initializer")
	require.Empty(t, hash)
}

func TestInitializeCloudCancellation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cloud initialization executes Linux bash")
	}
	dir := t.TempDir()
	nbcPath := filepath.Join(dir, "nbc.sh")
	scriptPath := filepath.Join(dir, "init.sh")
	require.NoError(t, os.WriteFile(nbcPath, []byte("LOCATION=ussec"), 0600))
	require.NoError(t, os.WriteFile(scriptPath, []byte("exit 0\n"), 0600))
	app := &App{cmdRun: func(cmd *exec.Cmd) error { return cmd.Run() }}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	hash, err := app.initializeCloud(ctx, ProvisionFlags{NBCCmd: nbcPath}, scriptPath)
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, hash)
}

func TestCloudInitializationHandoffAfterScriptReplacement(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cloud initialization executes Linux bash")
	}
	dir := t.TempDir()
	nbcPath := filepath.Join(dir, "nbc.sh")
	scriptPath := filepath.Join(dir, "init.sh")
	outputPath := filepath.Join(dir, "initializations")
	helpersPath, err := filepath.Abs("../parts/linux/cloud-init/artifacts/init-aks-cloud.sh")
	require.NoError(t, err)
	script := fmt.Sprintf(`#!/bin/bash
__SOURCED__=1 . %q
if cloud_init_already_done "${1:-init}" "$0"; then exit 0; fi
printf 'initialized\n' >> "$CLOUD_INIT_TEST_OUTPUT"
`, helpersPath)
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0700))
	require.NoError(t, os.WriteFile(nbcPath, []byte(
		fmt.Sprintf("LOCATION=ussec REPO_DEPOT_ENDPOINT=https://depot %q", scriptPath)), 0600))
	t.Setenv("CLOUD_INIT_TEST_OUTPUT", outputPath)
	app := &App{cmdRun: cmdRunner}
	flags := ProvisionFlags{NBCCmd: nbcPath}
	hash, err := app.initializeCloud(t.Context(), flags, scriptPath)
	require.NoError(t, err)
	t.Setenv("AKS_CLOUD_INIT_SHA256", hash)

	_, err = app.Provision(t.Context(), flags)
	require.NoError(t, err)
	initializations, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "initialized\n", string(initializations))

	require.NoError(t, os.WriteFile(scriptPath, []byte(script+"\n# hotfix replacement\n"), 0700))
	_, err = app.Provision(t.Context(), flags)
	require.NoError(t, err)
	initializations, err = os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Equal(t, "initialized\ninitialized\n", string(initializations))
}

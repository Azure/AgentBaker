package main

import (
	"context"
	"errors"
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
	require.NoError(t, os.WriteFile(configPath, []byte(`{"cluster_config":{"location":"config-region"},"custom_cloud_config":{"custom_cloud_env_name":"AKSCustom","repo_depot_endpoint":"https://config"}}`), 0600))

	t.Run("NBC takes precedence and preserves shell-quoted values", func(t *testing.T) {
		nbc := nbcCloudInitGuard + `
 REPO_DEPOT_ENDPOINT='https://nbc/'"'"'$(touch nope)' LOCATION='usnat test' "${INIT_AKS_CLOUD_FILEPATH}";
fi;
LOCATION='usnat test' /usr/bin/nohup /bin/bash
`
		require.NoError(t, os.WriteFile(nbcPath, []byte(nbc), 0600))
		location, endpoint, supported, err := cloudInitializationSettings(ProvisionFlags{ProvisionConfig: configPath, NBCCmd: nbcPath})
		require.NoError(t, err)
		require.True(t, supported)
		require.Equal(t, "usnat test", location)
		require.Equal(t, "https://nbc/'$(touch nope)", endpoint)
	})
	t.Run("older NBC retains old ordering even when config is present", func(t *testing.T) {
		require.NoError(t, os.WriteFile(nbcPath, []byte(`REPO_DEPOT_ENDPOINT="https://old" LOCATION=old "${INIT_AKS_CLOUD_FILEPATH}";`), 0600))
		_, _, supported, err := cloudInitializationSettings(ProvisionFlags{ProvisionConfig: configPath, NBCCmd: nbcPath})
		require.NoError(t, err)
		require.False(t, supported)
	})
	t.Run("config-only custom cloud", func(t *testing.T) {
		location, endpoint, supported, err := cloudInitializationSettings(ProvisionFlags{ProvisionConfig: configPath})
		require.NoError(t, err)
		require.True(t, supported)
		require.Equal(t, "config-region", location)
		require.Equal(t, "https://config", endpoint)
	})
	t.Run("non-custom cloud ignores endpoint", func(t *testing.T) {
		path := filepath.Join(dir, "public.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"clusterConfig":{"location":"eastus"},"customCloudConfig":{"customCloudEnvName":"public","repoDepotEndpoint":"https://ignored"}}`), 0600))
		location, endpoint, supported, err := cloudInitializationSettings(ProvisionFlags{ProvisionConfig: path})
		require.NoError(t, err)
		require.True(t, supported)
		require.Equal(t, "eastus", location)
		require.Empty(t, endpoint)
	})
	t.Run("empty settings are passed through", func(t *testing.T) {
		require.NoError(t, os.WriteFile(nbcPath, []byte(nbcCloudInitGuard+` REPO_DEPOT_ENDPOINT='' LOCATION='' "${INIT_AKS_CLOUD_FILEPATH}"; fi; LOCATION=''`), 0600))
		location, endpoint, supported, err := cloudInitializationSettings(ProvisionFlags{NBCCmd: nbcPath})
		require.NoError(t, err)
		require.True(t, supported)
		require.Empty(t, location)
		require.Empty(t, endpoint)
	})
	t.Run("missing settings fail instead of using config", func(t *testing.T) {
		require.NoError(t, os.WriteFile(nbcPath, []byte(nbcCloudInitGuard+` LOCATION='usnat'`), 0600))
		_, _, _, err := cloudInitializationSettings(ProvisionFlags{ProvisionConfig: configPath, NBCCmd: nbcPath})
		require.ErrorContains(t, err, "missing cloud initialization settings")
	})
	t.Run("invalid config fails", func(t *testing.T) {
		path := filepath.Join(dir, "invalid.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"cluster_config":{"location":{}}}`), 0600))
		_, _, _, err := cloudInitializationSettings(ProvisionFlags{ProvisionConfig: path})
		require.ErrorContains(t, err, "decode provision config")
	})
	t.Run("missing inputs fail", func(t *testing.T) {
		_, _, _, err := cloudInitializationSettings(ProvisionFlags{})
		require.Error(t, err)
		_, _, _, err = cloudInitializationSettings(ProvisionFlags{NBCCmd: dir})
		require.ErrorContains(t, err, "regular file")
	})
}

func TestInitializeCloudHandoff(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	markerPath := filepath.Join(dir, "run", "init-aks-cloud.done")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"cluster_config":{"location":"usnat"},"custom_cloud_config":{"custom_cloud_env_name":"AKSCustom","repo_depot_endpoint":"https://repo"}}`), 0600))
	flags := ProvisionFlags{ProvisionConfig: configPath}

	t.Run("success passes settings and creates marker", func(t *testing.T) {
		app := &App{cmdRun: func(cmd *exec.Cmd) error {
			env := envSliceToMap(cmd.Env)
			require.Equal(t, "usnat", env["LOCATION"])
			require.Equal(t, "https://repo", env["REPO_DEPOT_ENDPOINT"])
			_, err := os.Stat(markerPath)
			require.True(t, os.IsNotExist(err))
			return nil
		}}
		require.NoError(t, app.initializeCloud(context.Background(), flags, "/initializer", markerPath))
		_, err := os.Stat(markerPath)
		require.NoError(t, err)
		require.NoError(t, os.Remove(markerPath))
	})
	t.Run("failure does not create marker", func(t *testing.T) {
		app := &App{cmdRun: func(*exec.Cmd) error { return errors.New("initializer failed") }}
		require.ErrorContains(t, app.initializeCloud(context.Background(), flags, "/initializer", markerPath), "initializer failed")
		_, err := os.Stat(markerPath)
		require.True(t, os.IsNotExist(err))
	})
	t.Run("missing initializer fails without a marker", func(t *testing.T) {
		app := &App{cmdRun: cmdRunner}
		require.ErrorContains(t, app.initializeCloud(context.Background(), flags, filepath.Join(dir, "missing"), markerPath), "initialize cloud")
		_, err := os.Stat(markerPath)
		require.True(t, os.IsNotExist(err))
	})
	t.Run("marker failure does not invalidate successful initialization", func(t *testing.T) {
		blocker := filepath.Join(dir, "not-a-directory")
		require.NoError(t, os.WriteFile(blocker, nil, 0600))
		app := &App{cmdRun: func(*exec.Cmd) error { return nil }}
		require.NoError(t, app.initializeCloud(context.Background(), flags, "/initializer", filepath.Join(blocker, "done")))
	})
	t.Run("old NBC does not run initializer or create marker", func(t *testing.T) {
		path := filepath.Join(dir, "old.sh")
		require.NoError(t, os.WriteFile(path, []byte("LOCATION=usnat"), 0600))
		app := &App{cmdRun: func(*exec.Cmd) error { t.Fatal("unexpected initialization"); return nil }}
		require.NoError(t, app.initializeCloud(context.Background(), ProvisionFlags{NBCCmd: path}, "/initializer", markerPath))
		_, err := os.Stat(markerPath)
		require.True(t, os.IsNotExist(err))
	})
}

func TestInitializeCloudExecutesWithLiteralNBCSettings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}
	dir := t.TempDir()
	nbcPath := filepath.Join(dir, "nbc.sh")
	scriptPath := filepath.Join(dir, "init.sh")
	resultPath := filepath.Join(dir, "result")
	markerPath := filepath.Join(dir, "run", "done")
	nbc := nbcCloudInitGuard + ` REPO_DEPOT_ENDPOINT='https://repo/'"'"'$(touch nope)' LOCATION='usnat' "${INIT_AKS_CLOUD_FILEPATH}"; fi; LOCATION='usnat'`
	require.NoError(t, os.WriteFile(nbcPath, []byte(nbc), 0600))
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\nprintf '%s\\n%s\\n' \"$LOCATION\" \"$REPO_DEPOT_ENDPOINT\" > \"$RESULT\"\n"), 0700))
	t.Setenv("RESULT", resultPath)
	app := &App{cmdRun: cmdRunner}
	require.NoError(t, app.initializeCloud(context.Background(), ProvisionFlags{NBCCmd: nbcPath}, scriptPath, markerPath))
	result, err := os.ReadFile(resultPath)
	require.NoError(t, err)
	require.Equal(t, "usnat\nhttps://repo/'$(touch nope)\n", string(result))
	_, err = os.Stat(markerPath)
	require.NoError(t, err)
}

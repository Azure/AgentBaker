package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudPrerequisiteSettings(t *testing.T) {
	for _, tc := range []struct {
		name, custom, endpoint string
	}{
		{"public", "", ""},
		{"custom", `,"custom_cloud_config":{"custom_cloud_env_name":"akscustom","repo_depot_endpoint":"https://depot/ubuntu"}`, "https://depot/ubuntu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			require.NoError(t, os.WriteFile(path, []byte(`{`+requiredProvisionFields+`"version":"v1"`+tc.custom+`}`), 0600))
			settings, err := cloudPrerequisiteSettings(ProvisionFlags{ProvisionConfig: path})
			require.NoError(t, err)
			require.Equal(t, "westus2", settings["LOCATION"])
			require.Equal(t, tc.endpoint, settings["REPO_DEPOT_ENDPOINT"])
		})
	}

	t.Run("NBC wins and is parsed without execution", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nbc.sh")
		touched := filepath.Join(dir, "executed")
		input := `LOCATION="ussec east"; REPO_DEPOT_ENDPOINT='https://live-depot/ubuntu'; touch '` + touched + `'`
		require.NoError(t, os.WriteFile(path, []byte(input), 0600))
		settings, err := cloudPrerequisiteSettings(ProvisionFlags{ProvisionConfig: "must-not-read", NBCCmd: path})
		require.NoError(t, err)
		require.Equal(t, "ussec east", settings["LOCATION"])
		require.Equal(t, "https://live-depot/ubuntu", settings["REPO_DEPOT_ENDPOINT"])
		require.NoFileExists(t, touched)
	})

	t.Run("NBC missing location fails explicitly", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nbc.sh")
		require.NoError(t, os.WriteFile(path, []byte("REPO_DEPOT_ENDPOINT=https://depot"), 0600))
		_, err := cloudPrerequisiteSettings(ProvisionFlags{NBCCmd: path})
		require.ErrorContains(t, err, "must include LOCATION")
	})

	t.Run("NBC must be a regular file", func(t *testing.T) {
		_, err := cloudPrerequisiteSettings(ProvisionFlags{NBCCmd: t.TempDir()})
		require.ErrorContains(t, err, "must be a regular file")
	})

	for _, flags := range []ProvisionFlags{
		{},
		{NBCCmd: "missing"},
		{ProvisionConfig: "missing"},
	} {
		_, err := cloudPrerequisiteSettings(flags)
		require.Error(t, err)
	}
}

func TestInitializeCloudPrerequisites(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "certificate failure"}[fail], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nbc.sh")
			require.NoError(t, os.WriteFile(path, []byte(`LOCATION=usnat REPO_DEPOT_ENDPOINT="https://depot"`), 0600))
			t.Setenv("LOCATION", "baked-location")
			t.Setenv("REPO_DEPOT_ENDPOINT", "https://baked-depot")
			t.Setenv("AKS_CLOUD_PREREQUISITES_DONE", "true")
			calls := 0
			app := &App{initAKSCloudPath: "/test/init-aks-cloud.sh", cmdRun: func(cmd *exec.Cmd) error {
				calls++
				require.Equal(t, []string{"/bin/bash", "/test/init-aks-cloud.sh", "prerequisites"}, cmd.Args)
				env := envSliceToMap(cmd.Env)
				require.Equal(t, "usnat", env["LOCATION"])
				require.Equal(t, "https://depot", env["REPO_DEPOT_ENDPOINT"])
				require.Equal(t, "false", env["AKS_CLOUD_PREREQUISITES_DONE"])
				if fail {
					return errors.New("certificate bootstrap failed")
				}

				return nil
			}}
			result, err := app.initializeCloudPrerequisites(context.Background(), ProvisionFlags{NBCCmd: path})
			require.Equal(t, 1, calls)
			if fail {
				require.ErrorContains(t, err, "certificate bootstrap failed")
				require.Equal(t, "240", result.ExitCode)
				require.Contains(t, result.Error, "initialize cloud prerequisites")
			} else {
				require.NoError(t, err)
				require.Equal(t, "0", result.ExitCode)
			}
		})
	}
}

func TestCloudPrerequisiteFailureSignalsProvisioning(t *testing.T) {
	for _, invalidInput := range []bool{false, true} {
		t.Run(map[bool]string{false: "certificate failure", true: "invalid config"}[invalidInput], func(t *testing.T) {
			tt := NewTestApp(t, TestAppConfig{RunFunc: func(*exec.Cmd) error {
				return errors.New("required certificate bootstrap failed")
			}})
			dir := t.TempDir()
			tt.App.provisionStatusFiles = &ProvisionStatusFiles{
				ProvisionJSONFile:     filepath.Join(dir, "provision.json"),
				ProvisionCompleteFile: filepath.Join(dir, "provision.complete"),
			}
			require.NoError(t, os.WriteFile(tt.App.readyReportMarkerPath, nil, 0600))
			reports := 0
			tt.App.goalStateReporter = func(_ context.Context, failed bool, description string) error {
				reports++
				require.True(t, failed)
				require.NotEmpty(t, description)
				return nil
			}
			args := []string{"anc", "cloud-prerequisites"}
			if invalidInput {
				args = append(args, "--provision-config=missing")
			} else {
				path := filepath.Join(dir, "nbc.sh")
				require.NoError(t, os.WriteFile(path, []byte("LOCATION=ussec"), 0600))
				args = append(args, "--nbc-cmd="+path)
			}
			require.NotZero(t, tt.App.Run(t.Context(), args))
			require.Equal(t, 1, reports)
			require.FileExists(t, tt.App.provisionStatusFiles.ProvisionCompleteFile)
			data, err := os.ReadFile(tt.App.provisionStatusFiles.ProvisionJSONFile)
			require.NoError(t, err)
			var result ProvisionResult
			require.NoError(t, json.Unmarshal(data, &result))
			require.Equal(t, "240", result.ExitCode)
			require.NotEmpty(t, result.Error)
			_, err = tt.App.ProvisionWait(t.Context(), *tt.App.provisionStatusFiles)
			require.Error(t, err)
		})
	}
}

func TestCloudPrerequisiteProcessFailure(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cloud initialization executes Linux bash")
	}
	dir := t.TempDir()
	nbcPath := filepath.Join(dir, "nbc.sh")
	scriptPath := filepath.Join(dir, "init.sh")
	require.NoError(t, os.WriteFile(nbcPath, []byte("LOCATION=ussec REPO_DEPOT_ENDPOINT=https://depot"), 0600))
	script := "printf '%s\\n' \"$1:$LOCATION:$REPO_DEPOT_ENDPOINT\"\necho 'certificate installation failed' >&2\nexit 23\n"
	require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0600))
	app := &App{initAKSCloudPath: scriptPath, cmdRun: cmdRunner}
	result, err := app.initializeCloudPrerequisites(t.Context(), ProvisionFlags{NBCCmd: nbcPath})
	require.Error(t, err)
	require.Equal(t, "23", result.ExitCode)
	require.Contains(t, result.Output, "prerequisites:ussec:https://depot")
	require.Contains(t, result.Output, "certificate installation failed")
}

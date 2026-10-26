package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func writeCloudProvisionConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(`{`+requiredProvisionFields+`"version":"v1"}`), 0o600))
	return path
}

func requireCloudInitializationAllowsProvisioning(t *testing.T, c *cloudInitializer) {
	t.Helper()
	called := false
	tt := NewTestApp(t, TestAppConfig{RunFunc: func(cmd *exec.Cmd) error {
		called = true
		require.Contains(t, cmd.String(), "/opt/azure/containers/provision_start.sh")
		path, lookupErr := exec.LookPath("true")
		require.NoError(t, lookupErr)
		// Execute a harmless stub so Provision observes a real zero exit status.
		cmd.Path, cmd.Args = path, []string{path}
		return cmd.Run()
	}})
	tt.App.initializeCloud = func(ctx context.Context, config cloudInitConfig) error {
		config.RepoDepotEndpoint = c.wireEndpoint
		return c.initialize(ctx, config)
	}
	result, err := tt.App.Provision(t.Context(), ProvisionFlags{ProvisionConfig: writeCloudProvisionConfig(t)})
	require.NoError(t, err)
	require.True(t, called)
	require.Equal(t, "0", result.ExitCode)
}

func TestNativeCloudCronFailuresDoNotBlockProvisioning(t *testing.T) {
	for _, failedArg := range []string{"-l", "-"} {
		t.Run(failedArg, func(t *testing.T) {
			c := newTestCloudInitializer(t, cloudTestUbuntu)
			serveCloudTestResponses(t, c, cloudTestCertificate(t), `{"IsOptedInForRootCerts":true}`)
			run := c.run
			writes, cacheUpdates := 0, 0
			c.run = func(cmd *exec.Cmd) error {
				if cmd.Args[0] == "crontab" {
					if cmd.Args[1] == "-" {
						writes++
					}
					if cmd.Args[1] == failedArg {
						return errors.New("crontab operation failed")
					}
				}
				if cmd.Args[0] == "apt-get" {
					cacheUpdates++
				}
				return run(cmd)
			}
			requireCloudInitializationAllowsProvisioning(t, c)
			require.Equal(t, 1, writes)
			require.Equal(t, 1, cacheUpdates)
		})
	}
}

func TestNativeCloudCacheFailuresDoNotBlockProvisioning(t *testing.T) {
	for _, tc := range []struct{ id, release, command string }{
		{osReleaseIDUbuntu, cloudTestUbuntu, "apt-get"},
		{osReleaseIDMariner, "ID=mariner\n", "dnf"},
		{osReleaseIDAzureLinux, "ID=azurelinux\n", "dnf"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			c := newTestCloudInitializer(t, tc.release)
			serveCloudTestResponses(t, c, cloudTestCertificate(t), `{"IsOptedInForRootCerts":true}`)
			if tc.id == osReleaseIDMariner {
				require.NoError(t, c.writeFile(defaultYumReposDir+"/mariner-extras.repo",
					[]byte("[mariner-extras]\nbaseurl=https://packages.microsoft.com/extras\n")))
			}
			run := c.run
			attempts := 0
			c.run = func(cmd *exec.Cmd) error {
				if cmd.Args[0] == tc.command {
					attempts++
					return errors.New("cache refresh failed")
				}
				return run(cmd)
			}
			requireCloudInitializationAllowsProvisioning(t, c)
			require.Equal(t, cloudRequestAttempts, attempts)
			levels := make(map[string]string)
			for _, event := range c.eventLogger.Events() {
				levels[event.TaskName] = event.EventLevel
			}
			require.Equal(t, "Error", levels["AKS.AKSNodeController.CloudRepositoryInitialization"])
			require.Equal(t, "Informational", levels["AKS.AKSNodeController.CloudInitialization"])
		})
	}
}

func TestNativeCloudCacheCancellationStillBlocksProvisioning(t *testing.T) {
	c := newTestCloudInitializer(t, "ID=azurelinux\n")
	serveCloudTestResponses(t, c, nil, `{"IsOptedInForRootCerts":false}`)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	c.run = func(cmd *exec.Cmd) error {
		require.Equal(t, "dnf", cmd.Args[0])
		cancel()
		return ctx.Err()
	}
	tt := NewTestApp(t, TestAppConfig{RunFunc: func(*exec.Cmd) error {
		t.Fatal("provisioning must not run after initialization is canceled")
		return nil
	}})
	tt.App.initializeCloud = func(ctx context.Context, config cloudInitConfig) error {
		config.RepoDepotEndpoint = c.wireEndpoint
		return c.initialize(ctx, config)
	}
	result, err := tt.App.Provision(ctx, ProvisionFlags{ProvisionConfig: writeCloudProvisionConfig(t)})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, "240", result.ExitCode)
}

func TestNativeCloudInitializationPrecedesProvisioning(t *testing.T) {
	var order []string
	tt := NewTestApp(t, TestAppConfig{RunFunc: func(*exec.Cmd) error {
		order = append(order, "provision")
		return nil
	}})
	tt.App.initializeCloud = func(_ context.Context, config cloudInitConfig) error {
		order = append(order, "initialize")
		require.Equal(t, "westus2", config.Location)
		require.Equal(t, 15*time.Minute, config.Timeout)
		return nil
	}
	_, err := tt.App.Provision(t.Context(), ProvisionFlags{ProvisionConfig: writeCloudProvisionConfig(t)})
	require.NoError(t, err)
	require.Equal(t, []string{"initialize", "provision"}, order)
}

func TestNativeCloudInitializationInstallsCertificatesBeforeProvisioning(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	cert := cloudTestCertificate(t)
	serveCloudTestResponses(t, c, cert, `{"IsOptedInForRootCerts":true}`)
	called := false
	tt := NewTestApp(t, TestAppConfig{RunFunc: func(*exec.Cmd) error {
		called = true
		installed, err := os.ReadFile(c.path("/usr/local/share/ca-certificates/root.crt"))
		require.NoError(t, err)
		require.Equal(t, cert, installed)
		return nil
	}})
	tt.App.initializeCloud = c.initialize
	_, err := tt.App.Provision(t.Context(), ProvisionFlags{ProvisionConfig: writeCloudProvisionConfig(t)})
	require.NoError(t, err)
	require.True(t, called)
}

func TestNativeCloudInitializationFailureReportsWithoutRunningScripts(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{RunFunc: func(*exec.Cmd) error {
		t.Fatal("provisioning must not run after cloud initialization fails")
		return nil
	}})
	tt.App.initializeCloud = func(context.Context, cloudInitConfig) error { return errors.New("cloud initialization failed") }
	require.NoError(t, os.WriteFile(tt.App.readyReportMarkerPath, nil, 0o600))
	reported := false
	tt.App.goalStateReporter = func(_ context.Context, failed bool, description string) error {
		reported = true
		require.True(t, failed)
		require.Contains(t, description, "cloud initialization failed")
		return nil
	}
	result, err := tt.App.runProvision(t.Context(), ProvisionFlags{ProvisionConfig: writeCloudProvisionConfig(t)}, false)
	require.ErrorContains(t, err, "cloud initialization failed")
	require.Equal(t, "240", result.ExitCode)
	require.Equal(t, err.Error(), result.Error)
	require.True(t, reported)
}

func TestNativeCloudInitializationIsNotUsedForNBC(t *testing.T) {
	for _, compare := range []bool{false, true} {
		t.Run(map[bool]string{false: "phase2", true: "phase2.5"}[compare], func(t *testing.T) {
			tt := NewTestApp(t, TestAppConfig{})
			tt.App.initializeCloud = func(context.Context, cloudInitConfig) error {
				t.Fatal("NBC must retain shell initialization")
				return nil
			}
			path := filepath.Join(t.TempDir(), "nbc-cmd.sh")
			require.NoError(t, os.WriteFile(path, []byte("#!/bin/bash\ntrue\n"), 0o600))
			flags := ProvisionFlags{NBCCmd: path}
			if compare {
				flags.ProvisionConfig = writeCloudProvisionConfig(t)
			}
			_, err := tt.App.Provision(t.Context(), flags)
			require.NoError(t, err)
		})
	}
}

func TestNativeCloudInitializationDryRunHasNoSideEffects(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{RunFunc: func(*exec.Cmd) error {
		t.Fatal("dry-run must not run commands")
		return nil
	}})
	tt.App.initializeCloud = func(context.Context, cloudInitConfig) error {
		t.Fatal("dry-run must not fetch certificates or modify the host")
		return nil
	}
	_, err := tt.App.runProvision(t.Context(), ProvisionFlags{ProvisionConfig: writeCloudProvisionConfig(t)}, true)
	require.NoError(t, err)
}

func TestNativeCloudInitializationConfigEnvironment(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	tt.App.initializeCloud = func(_ context.Context, config cloudInitConfig) error {
		require.Equal(t, cloudInitConfig{
			Location: "usnateast", RepoDepotEndpoint: "https://repo.example/ubuntu", Timeout: 30 * time.Minute,
		}, config)
		return nil
	}
	require.NoError(t, tt.App.initializeAKSCloud(t.Context(), map[string]string{
		"LOCATION": "usnateast", "REPO_DEPOT_ENDPOINT": "https://repo.example/ubuntu", "CSE_TIMEOUT": "1800",
	}))
	for _, timeout := range []string{"", "bad", "0", "-1"} {
		require.ErrorContains(t, tt.App.initializeAKSCloud(t.Context(), map[string]string{"CSE_TIMEOUT": timeout}), "invalid cloud initialization timeout")
	}
}

func TestCloudInitializationExpectedNBCEnvironmentDifference(t *testing.T) {
	require.Empty(t, diffEnvMaps(nil, map[string]string{"INIT_AKS_CLOUD_FILEPATH": "/opt/azure/containers/init-aks-cloud.sh"}))
}

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

func TestCloudCertificateRefreshCommand(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		err       error
		wantCall  bool
		wantError bool
	}{
		{"valid", []string{"--location", "usnateast"}, nil, true, false},
		{"refresh failure", []string{"--location", "usnateast"}, errors.New("refresh failed"), true, true},
		{"missing location", nil, nil, false, true},
		{"empty location", []string{"--location", " "}, nil, false, true},
		{"unexpected argument", []string{"--location", "usnateast", "extra"}, nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tt := NewTestApp(t, TestAppConfig{})
			called := false
			tt.App.refreshCloudCertificates = func(_ context.Context, location string) error {
				called = true
				require.Equal(t, "usnateast", location)
				return tc.err
			}
			code := tt.App.Run(t.Context(), append([]string{"aks-node-controller", "refresh-cloud-certificates"}, tc.args...))
			require.Equal(t, tc.wantCall, called)
			require.Equal(t, tc.wantError, code != 0)
		})
	}
}

func TestCloudInitializationExpectedNBCEnvironmentDifference(t *testing.T) {
	require.Empty(t, diffEnvMaps(nil, map[string]string{"INIT_AKS_CLOUD_FILEPATH": "/opt/azure/containers/init-aks-cloud.sh"}))
}

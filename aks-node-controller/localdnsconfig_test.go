package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeLocalDNSTestNodeConfig(t *testing.T, app *App) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "aks-node-controller-config.json")
	require.NoError(t, os.WriteFile(p, []byte(fmt.Sprintf(`{
  "version": "v1",
  "kubelet_config": {
    "kubelet_node_labels": {
      "kubernetes.azure.com/agentpool": %q
    }
  }
}`, "pool1")), 0o600))
	app.nodeConfigPath = p
}

func TestFetchAndApplyLocalDNSConfig(t *testing.T) {
	t.Run("corefileBase64 rewrites output", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		want := ".:53 {\n    forward . 168.63.129.16\n}\n"
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{"corefileBase64":"` + base64.StdEncoding.EncodeToString([]byte(want)) + `"}`, nil
		}
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigApplied, outcome)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
	})

	t.Run("agent pool corefileBase64 rewrites output", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		want := ".:53 {\n    forward . 168.63.129.16\n    reload\n}\n"
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{"agentPools":{"pool1":{"corefileVersion":"abc123","corefileBase64":"` +
				base64.StdEncoding.EncodeToString([]byte(want)) + `"},"pool2":{"corefileBase64":"ignored"}}}`, nil
		}
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigApplied, outcome)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
		version, err := os.ReadFile(localDNSCorefileVersionPath(out))
		require.NoError(t, err)
		assert.Equal(t, "abc123\n", string(version))
	})

	t.Run("already current version skips rewrite", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		original := ".:53 {\n    forward . 1.1.1.1\n}\n"
		require.NoError(t, os.WriteFile(out, []byte(original), 0o644))
		require.NoError(t, writeLocalDNSCorefileVersion(localDNSCorefileVersionPath(out), "abc123"))
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{"agentPools":{"pool1":{"corefileVersion":"abc123","corefileBase64":"` +
				base64.StdEncoding.EncodeToString([]byte("should not be written")) + `"}}}`, nil
		}

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigAlreadyCurrent, outcome)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, original, string(got))
	})

	t.Run("agent pool version only config is no-op", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{"agentPools":{"pool1":{"corefileVersion":"abc123"}}}`, nil
		}
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigNoCorefileData, outcome)
		_, statErr := os.Stat(out)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("agent pool localDnsProfile renders corefile", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{
  "agentPools": {
    "pool1": {
      "corefileVersion": "profile-hash",
      "localDnsProfile": {
        "enableLocalDns": true,
        "vnetDnsOverrides": {
          ".": {
            "queryLogging": "Error",
            "protocol": "PreferUDP",
            "forwardDestination": "VnetDNS",
            "forwardPolicy": "Sequential",
            "maxConcurrent": 1000,
            "cacheDurationInSeconds": 3600,
            "serveStaleDurationInSeconds": 3600,
            "serveStale": "Immediate"
          }
        },
        "kubeDnsOverrides": {
          "cluster.local": {
            "queryLogging": "Error",
            "protocol": "PreferUDP",
            "forwardDestination": "ClusterCoreDNS",
            "forwardPolicy": "Sequential",
            "maxConcurrent": 1000,
            "cacheDurationInSeconds": 3600,
            "serveStaleDurationInSeconds": 3600,
            "serveStale": "Immediate"
          }
        }
      }
    }
  }
}`, nil
		}
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigApplied, outcome)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Contains(t, string(got), "health-check.localdns.local:53")
		assert.Contains(t, string(got), "cluster.local:53")
		version, err := os.ReadFile(localDNSCorefileVersionPath(out))
		require.NoError(t, err)
		assert.Equal(t, "profile-hash\n", string(version))
	})

	t.Run("other agent pool config is no-op", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{"agentPools":{"pool2":{"corefileBase64":"` + base64.StdEncoding.EncodeToString([]byte("ignored")) + `"}}}`, nil
		}
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigNoCorefileData, outcome)
		_, statErr := os.Stat(out)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("cli action fails open", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return "", assert.AnError
		}
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "fetch-localdns-config", "--output", filepath.Join(t.TempDir(), "localdns.corefile")})
		assert.Equal(t, 0, exitCode)
	})
}

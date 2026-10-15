package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func writeLocalDNSTestNodeConfig(t *testing.T, app *App) {
	t.Helper()
	writeLocalDNSTestNodeConfigWithCoreDNSIP(t, app, "")
}

func writeLocalDNSTestNodeConfigWithCoreDNSIP(t *testing.T, app *App, coreDNSServiceIP string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "aks-node-controller-config.json")
	require.NoError(t, os.WriteFile(p, []byte(fmt.Sprintf(`{
  "version": "v1",
  "cluster_config": {
    "cluster_network_config": {
      "core_dns_service_ip": %q
    }
  },
  "kubelet_config": {
    "kubelet_node_labels": {
      "kubernetes.azure.com/agentpool": %q
    }
  }
}`, coreDNSServiceIP, "pool1")), 0o600))
	app.nodeConfigPath = p
}

// writeLocalDNSTestBakedCorefile stands in for the localdns.corefile localdns.sh regenerates at
// startup, which the render reads bootstrap's CoreDNS service IP and hosts plugin choice back from.
func writeLocalDNSTestBakedCorefile(t *testing.T, app *App, corefile string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "localdns.corefile")
	require.NoError(t, os.WriteFile(p, []byte(corefile), 0o644))
	app.localDNSBakedCorefilePath = p
}

// bakedLocalDNSTestCorefile renders a baked Corefile whose cluster.local block forwards to 10.20.0.10.
func bakedLocalDNSTestCorefile(withHosts bool) string {
	hosts := ""
	if withHosts {
		hosts = "    hosts /etc/localdns/hosts {\n        fallthrough\n    }\n"
	}
	return ".:53 {\n    bind 169.254.10.10\n" + hosts + "    forward . 168.63.129.16 {\n    }\n}\n" +
		"cluster.local:53 {\n    bind 169.254.10.11\n    forward . 10.20.0.10 {\n    }\n}\n"
}

const localDNSTestProfilePayload = `{
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
}`

func TestFetchAndApplyLocalDNSConfig(t *testing.T) {
	t.Run("corefileBase64 rewrites output", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		want := ".:53 {\n    forward . 168.63.129.16\n}\n"
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{"corefileBase64":"` + base64.StdEncoding.EncodeToString([]byte(want)) + `"}`, nil
		}
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out, tt.App.fetchLocalDNSConfig)
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

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out, tt.App.fetchLocalDNSConfig)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigApplied, outcome)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
		version, err := os.ReadFile(localDNSCorefileVersionPath(out))
		require.NoError(t, err)
		assert.Equal(t, "abc123\n", string(version))
	})

	t.Run("already current version and content skips rewrite", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		original := ".:53 {\n    forward . 1.1.1.1\n}\n"
		require.NoError(t, os.WriteFile(out, []byte(original), 0o644))
		require.NoError(t, writeLocalDNSCorefileVersion(localDNSCorefileVersionPath(out), "abc123"))
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{"agentPools":{"pool1":{"corefileVersion":"abc123","corefileBase64":"` +
				base64.StdEncoding.EncodeToString([]byte(original)) + `"}}}`, nil
		}

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out, tt.App.fetchLocalDNSConfig)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigAlreadyCurrent, outcome)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, original, string(got))
	})

	t.Run("matching version with stale corefile rewrites output", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		out := filepath.Join(t.TempDir(), "localdns.corefile")
		require.NoError(t, os.WriteFile(out, []byte("stale-corefile"), 0o644))
		require.NoError(t, writeLocalDNSCorefileVersion(localDNSCorefileVersionPath(out), "abc123"))
		want := ".:53 {\n    forward . 168.63.129.16\n}\n"
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{"agentPools":{"pool1":{"corefileVersion":"abc123","corefileBase64":"` +
				base64.StdEncoding.EncodeToString([]byte(want)) + `"}}}`, nil
		}

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out, tt.App.fetchLocalDNSConfig)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigApplied, outcome)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
	})

	t.Run("agent pool version only config is no-op", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return `{"agentPools":{"pool1":{"corefileVersion":"abc123"}}}`, nil
		}
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out, tt.App.fetchLocalDNSConfig)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigNoCorefileData, outcome)
		_, statErr := os.Stat(out)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("agent pool localDnsProfile renders corefile", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		writeLocalDNSTestBakedCorefile(t, tt.App, bakedLocalDNSTestCorefile(false))
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return localDNSTestProfilePayload, nil
		}
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out, tt.App.fetchLocalDNSConfig)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigApplied, outcome)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Contains(t, string(got), "health-check.localdns.local:53")
		assert.Contains(t, string(got), "cluster.local:53")
		assert.Contains(t, string(got), "forward . 10.20.0.10")
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

		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out, tt.App.fetchLocalDNSConfig)
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

// TestLocalDNSRenderReusesBootstrapValues pins that the node-specific values the live-patching
// payload does not carry -- the CoreDNS service IP and whether the hosts plugin is on -- are read
// back from what bootstrap rendered, so a patch only changes what the profile changes.
func TestLocalDNSRenderReusesBootstrapValues(t *testing.T) {
	render := func(t *testing.T, tt *TestApp) (string, localDNSConfigOutcome, error) {
		t.Helper()
		tt.App.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return localDNSTestProfilePayload, nil
		}
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")
		outcome, err := tt.App.fetchAndApplyLocalDNSConfig(context.Background(), out, tt.App.fetchLocalDNSConfig)
		got, _ := os.ReadFile(out)
		return string(got), outcome, err
	}

	t.Run("baked corefile CoreDNS IP wins over node config", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfigWithCoreDNSIP(t, tt.App, "10.99.0.10")
		writeLocalDNSTestBakedCorefile(t, tt.App, bakedLocalDNSTestCorefile(false))

		got, outcome, err := render(t, tt)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigApplied, outcome)
		assert.Contains(t, got, "forward . 10.20.0.10")
		assert.NotContains(t, got, "10.99.0.10")
	})

	t.Run("node config CoreDNS IP is used without a baked corefile", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfigWithCoreDNSIP(t, tt.App, "10.99.0.10")
		tt.App.localDNSBakedCorefilePath = filepath.Join(t.TempDir(), "missing.corefile")

		got, outcome, err := render(t, tt)
		require.NoError(t, err)
		assert.Equal(t, outcomeLocalDNSConfigApplied, outcome)
		assert.Contains(t, got, "forward . 10.99.0.10")
	})

	t.Run("unknown CoreDNS IP refuses the render instead of defaulting", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		tt.App.localDNSBakedCorefilePath = filepath.Join(t.TempDir(), "missing.corefile")
		tt.App.nbcCmdPath = filepath.Join(t.TempDir(), "missing-nbc-cmd.sh")

		got, outcome, err := render(t, tt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CoreDNS service")
		assert.Equal(t, outcomeLocalDNSConfigFailed, outcome)
		assert.Empty(t, got, "nothing may be written when the CoreDNS IP is unknown")
	})

	t.Run("hosts plugin kept when the baked corefile has it", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		hostsFile := filepath.Join(t.TempDir(), "hosts")
		require.NoError(t, os.WriteFile(hostsFile, []byte("10.0.0.1 mcr.microsoft.com\n"), 0o644))
		tt.App.localDNSHostsFilePath = hostsFile
		writeLocalDNSTestBakedCorefile(t, tt.App,
			strings.ReplaceAll(bakedLocalDNSTestCorefile(true), "/etc/localdns/hosts", hostsFile))

		got, _, err := render(t, tt)
		require.NoError(t, err)
		// The template always names /etc/localdns/hosts; the test path only stands in for its presence.
		assert.Contains(t, got, "hosts /etc/localdns/hosts")
	})

	t.Run("hosts plugin stays off when the baked corefile has none, whatever nbc-cmd says", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App)
		hostsFile := filepath.Join(t.TempDir(), "hosts")
		require.NoError(t, os.WriteFile(hostsFile, []byte("10.0.0.1 mcr.microsoft.com\n"), 0o644))
		tt.App.localDNSHostsFilePath = hostsFile
		nbcCmd := filepath.Join(t.TempDir(), "nbc-cmd.sh")
		require.NoError(t, os.WriteFile(nbcCmd, []byte("SHOULD_ENABLE_HOSTS_PLUGIN=true /opt/azure/containers/provision_start.sh\n"), 0o600))
		tt.App.nbcCmdPath = nbcCmd
		writeLocalDNSTestBakedCorefile(t, tt.App, bakedLocalDNSTestCorefile(false))

		got, _, err := render(t, tt)
		require.NoError(t, err)
		assert.NotContains(t, got, "hosts /etc/localdns/hosts")
	})
}

// TestLocalDNSLPSUnavailable pins the contract the localDNS fetch path depends on: the shared
// mapGRPCError classifier must fold exactly the benign LPS codes into errLPSUnavailable, which is
// what turns into the notFound outcome. It guards against a change to mapGRPCError silently
// reclassifying one of these into a hard failure for localDNS.
func TestLocalDNSLPSUnavailable(t *testing.T) {
	tests := []struct {
		name string
		code codes.Code
		ok   bool
	}{
		{name: "not found", code: codes.NotFound, ok: true},
		{name: "permission denied", code: codes.PermissionDenied, ok: true},
		{name: "unauthenticated", code: codes.Unauthenticated, ok: true},
		{name: "internal", code: codes.Internal, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapped := mapGRPCError(status.Error(tt.code, "from LPS"))
			assert.Equal(t, tt.ok, isLPSUnavailable(mapped))
		})
	}
}

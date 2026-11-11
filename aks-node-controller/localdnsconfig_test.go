package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/aks-node-controller/internal/localdns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The render itself is tested in internal/localdns. These tests cover what the commands add: the
// output contract, fail-open, flag passing and how LPS errors map onto outcomes.

// localDNSTestPayload has one entry, for pool1 at version v1.
const localDNSTestPayload = `{"agentPools":{"pool1":{"corefileVersion":"v1","localDnsProfile":{"enableLocalDns":true,` +
	`"vnetDnsOverrides":{".":{"queryLogging":"Error","protocol":"PreferUDP","forwardDestination":"VnetDNS",` +
	`"forwardPolicy":"Sequential","maxConcurrent":1000,"cacheDurationInSeconds":7200,` +
	`"serveStaleDurationInSeconds":3600,"serveStale":"Immediate"}},` +
	`"kubeDnsOverrides":{"cluster.local":{"queryLogging":"Error","protocol":"PreferUDP",` +
	`"forwardDestination":"ClusterCoreDNS","forwardPolicy":"Sequential","maxConcurrent":1000,` +
	`"cacheDurationInSeconds":3600,"serveStaleDurationInSeconds":3600,"serveStale":"Immediate"}}}}}}`

// newLocalDNSTestApp is a node with neither provisioning file, as on a classic-CSE node, whose baked
// Corefile forwards cluster.local to 10.20.0.10.
func newLocalDNSTestApp(t *testing.T) *App {
	t.Helper()
	tt := NewTestApp(t, TestAppConfig{})
	tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "missing-aks-node-controller-config.json")
	tt.App.nbcCmdPath = filepath.Join(t.TempDir(), "missing-nbc-cmd.sh")
	baked := filepath.Join(t.TempDir(), "localdns.corefile")
	require.NoError(t, os.WriteFile(baked, []byte("cluster.local:53 {\n    forward . 10.20.0.10 {\n    }\n}\n"), 0o644))
	tt.App.localDNSBakedCorefilePath = baked
	return tt.App
}

func TestLocalDNSFetchConfigCommand(t *testing.T) {
	t.Run("fails open", func(t *testing.T) {
		app := newLocalDNSTestApp(t)
		app.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return "", assert.AnError
		}
		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "localdns", "fetch-config",
			"--output", filepath.Join(t.TempDir(), "localdns.corefile")})
		assert.Equal(t, 0, exitCode)
	})

	t.Run("renders this node's entry", func(t *testing.T) {
		app := newLocalDNSTestApp(t)
		nbcCmd := filepath.Join(t.TempDir(), "nbc-cmd.sh")
		require.NoError(t, os.WriteFile(nbcCmd, []byte("KUBELET_NODE_LABELS=kubernetes.azure.com/agentpool=pool1 /opt/azure/containers/provision_start.sh\n"), 0o600))
		app.nbcCmdPath = nbcCmd
		app.fetchLocalDNSConfigFn = func(context.Context) (string, error) { return localDNSTestPayload, nil }
		out := filepath.Join(t.TempDir(), "localdns.corefile")

		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "localdns", "fetch-config", "--output", out})
		assert.Equal(t, 0, exitCode)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Contains(t, string(got), "cache 7200 {")
	})

	t.Run("nothing published in LPS is not found", func(t *testing.T) {
		app := newLocalDNSTestApp(t)
		app.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return "", fmt.Errorf("get localDNS component config: %w", mapGRPCError(status.Error(codes.NotFound, "from LPS")))
		}
		result, err := localdns.Apply(context.Background(), app.fetchLocalDNSConfig,
			filepath.Join(t.TempDir(), "localdns.corefile"), "", app.localDNSSources())
		require.NoError(t, err)
		assert.Equal(t, localdns.OutcomeNotFound, result.Outcome)
	})

	t.Run("an LPS error is a failure", func(t *testing.T) {
		app := newLocalDNSTestApp(t)
		app.fetchLocalDNSConfigFn = func(context.Context) (string, error) {
			return "", fmt.Errorf("get localDNS component config: %w", mapGRPCError(status.Error(codes.Internal, "from LPS")))
		}
		result, err := localdns.Apply(context.Background(), app.fetchLocalDNSConfig,
			filepath.Join(t.TempDir(), "localdns.corefile"), "", app.localDNSSources())
		require.Error(t, err)
		assert.Equal(t, localdns.OutcomeFailed, result.Outcome)
	})
}

// TestLocalDNSApplyConfigCommand pins the output contract the localDNS handler parses: the outcome is
// always the last line, and on failure the reason precedes it as "error: <reason>".
func TestLocalDNSApplyConfigCommand(t *testing.T) {
	run := func(t *testing.T, app *App, agentPool string) ([]string, error) {
		t.Helper()
		configFile := filepath.Join(t.TempDir(), "localdns-config.json")
		require.NoError(t, os.WriteFile(configFile, []byte(localDNSTestPayload), 0o600))
		var out bytes.Buffer
		err := app.runApplyLocalDNSConfigCommand(context.Background(), configFile,
			filepath.Join(t.TempDir(), "livepatched.localdns.corefile"), agentPool, &out)
		return strings.Split(strings.TrimRight(out.String(), "\n"), "\n"), err
	}

	t.Run("failure prints the reason before the outcome", func(t *testing.T) {
		lines, err := run(t, newLocalDNSTestApp(t), "")
		require.Error(t, err)
		require.Len(t, lines, 2)
		assert.Equal(t, "error: "+err.Error(), lines[0])
		assert.Equal(t, string(localdns.OutcomeFailed), lines[1])
	})

	t.Run("success prints the version before the outcome", func(t *testing.T) {
		lines, err := run(t, newLocalDNSTestApp(t), "pool1")
		require.NoError(t, err)
		assert.Equal(t, []string{"version: v1", string(localdns.OutcomeRendered)}, lines)
	})

	t.Run("an unreadable config file is a failure", func(t *testing.T) {
		var out bytes.Buffer
		err := newLocalDNSTestApp(t).runApplyLocalDNSConfigCommand(context.Background(),
			filepath.Join(t.TempDir(), "missing.json"), filepath.Join(t.TempDir(), "out"), "pool1", &out)
		require.Error(t, err)
		assert.Equal(t, "error: "+err.Error()+"\nfailed\n", out.String())
	})

	t.Run("cli passes the agent pool through", func(t *testing.T) {
		app := newLocalDNSTestApp(t)
		configFile := filepath.Join(t.TempDir(), "localdns-config.json")
		require.NoError(t, os.WriteFile(configFile, []byte(localDNSTestPayload), 0o600))
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")

		exitCode := app.Run(context.Background(), []string{"aks-node-controller", "localdns", "apply-config",
			"--config-file", configFile, "--output", out, "--agent-pool", "pool1"})
		assert.Equal(t, 0, exitCode)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Contains(t, string(got), "cache 7200 {")
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

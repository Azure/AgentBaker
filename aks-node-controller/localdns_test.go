package main

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lpsv1 "github.com/Azure/agentbaker/aks-live-patching/pkg/gen/akslivepatching/v1"
	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The render itself is tested in internal/localdns. These tests cover the wiring the App supplies:
// the command names and flags, the shared LPS client, and the provisioning files.

// localDNSTestPayload has one entry, for pool1 at version v1.
const localDNSTestPayload = `{"agentPools":{"pool1":{"corefileVersion":"v1","localDnsProfile":{"enableLocalDns":true,` +
	`"vnetDnsOverrides":{".":{"queryLogging":"Error","protocol":"PreferUDP","forwardDestination":"VnetDNS",` +
	`"forwardPolicy":"Sequential","maxConcurrent":1000,"cacheDurationInSeconds":7200,` +
	`"serveStaleDurationInSeconds":3600,"serveStale":"Immediate"}},` +
	`"kubeDnsOverrides":{"cluster.local":{"queryLogging":"Error","protocol":"PreferUDP",` +
	`"forwardDestination":"ClusterCoreDNS","forwardPolicy":"Sequential","maxConcurrent":1000,` +
	`"cacheDurationInSeconds":3600,"serveStaleDurationInSeconds":3600,"serveStale":"Immediate"}}}}}}`

// writeLocalDNSTestNodeConfig gives the app a node config that names LPS, this node's pool (pool1)
// and the CoreDNS service IP, as scriptless provisioning writes it.
func writeLocalDNSTestNodeConfig(t *testing.T, app *App) {
	t.Helper()
	caB64 := base64.StdEncoding.EncodeToString([]byte(testCAPEM))
	nodeConfig := filepath.Join(t.TempDir(), "config.json")
	body := `{"version":"v1","api_server_config":{"api_server_name":"myapi.example.com"},"kubernetes_ca_cert":"` + caB64 + `",` +
		`"cluster_config":{"cluster_network_config":{"core_dns_service_ip":"10.20.0.10"}},` +
		`"kubelet_config":{"kubelet_node_labels":{"kubernetes.azure.com/agentpool":"pool1"}}}`
	require.NoError(t, os.WriteFile(nodeConfig, []byte(body), 0o644))
	app.nodeConfigPath = nodeConfig
}

// fetchLocalDNSConfigEvents returns the FetchLocalDNSConfig events as "<level> <message>".
func fetchLocalDNSConfigEvents(app *App) []string {
	var events []string
	for _, event := range app.eventLogger.Events() {
		if event.TaskName == "AKS.AKSNodeController.FetchLocalDNSConfig" {
			message, _, _ := strings.Cut(event.Message, " | startTime=")
			events = append(events, event.EventLevel+" "+message)
		}
	}
	return events
}

func TestLocalDNSFetchConfigCommand(t *testing.T) {
	fetch := func(t *testing.T, srv *mockLPSServer) (*App, string, int) {
		t.Helper()
		tt := newGRPCTestApp(t, srv)
		writeLocalDNSTestNodeConfig(t, tt.App)
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "localdns", "fetch-config", "--output", out})
		return tt.App, out, exitCode
	}

	t.Run("renders this node's entry from LPS", func(t *testing.T) {
		srv := &mockLPSServer{resp: &lpsv1.GetComponentConfigResponse{ComponentName: "localDNS", Config: localDNSTestPayload}}
		app, out, exitCode := fetch(t, srv)

		assert.Equal(t, 0, exitCode)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Contains(t, string(got), "cache 7200 {")
		assert.Contains(t, string(got), "forward . 10.20.0.10")
		assert.Equal(t, "localDNS", srv.gotComponent)
		assert.Equal(t, []string{"attested-doc-token"}, srv.gotToken)
		assert.Equal(t, []string{testVMResourceID}, srv.gotResourceID)
		assert.Equal(t, []string{string(helpers.EventLevelInformational) + " localdns fetch-config outcome=rendered"},
			fetchLocalDNSConfigEvents(app))
	})

	t.Run("nothing published in LPS is not found", func(t *testing.T) {
		app, out, exitCode := fetch(t, &mockLPSServer{err: status.Error(codes.NotFound, "no localDNS config")})

		assert.Equal(t, 0, exitCode)
		assert.NoFileExists(t, out)
		assert.Equal(t, []string{string(helpers.EventLevelInformational) + " localdns fetch-config outcome=notFound"},
			fetchLocalDNSConfigEvents(app))
	})

	t.Run("an LPS failure fails open", func(t *testing.T) {
		app, out, exitCode := fetch(t, &mockLPSServer{err: status.Error(codes.Internal, "boom")})

		assert.Equal(t, 0, exitCode)
		assert.NoFileExists(t, out)
		events := fetchLocalDNSConfigEvents(app)
		require.Len(t, events, 1)
		assert.True(t, strings.HasPrefix(events[0], string(helpers.EventLevelError)+" localdns fetch-config outcome=failed error="), events[0])
	})
}

func TestLocalDNSApplyConfigCommand(t *testing.T) {
	t.Run("passes the flags through", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		writeLocalDNSTestNodeConfig(t, tt.App) // names pool1; the flag picks pool2
		payload := strings.Replace(localDNSTestPayload, `"pool1"`, `"pool2"`, 1)
		configFile := filepath.Join(t.TempDir(), "localdns-config.json")
		require.NoError(t, os.WriteFile(configFile, []byte(payload), 0o600))
		out := filepath.Join(t.TempDir(), "livepatched.localdns.corefile")

		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "localdns", "apply-config",
			"--config-file", configFile, "--output", out, "--agent-pool", "pool2"})
		assert.Equal(t, 0, exitCode)
		got, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Contains(t, string(got), "cache 7200 {")
	})

	t.Run("unexpected arguments fail", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "localdns", "apply-config", "extra"})
		assert.NotEqual(t, 0, exitCode)
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

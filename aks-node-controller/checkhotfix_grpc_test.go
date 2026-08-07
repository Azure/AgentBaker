package main

import (
	"context"
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"testing"

	lpsv1 "github.com/Azure/agentbaker/aks-live-patching/pkg/gen/akslivepatching/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// mockLPSServer is an in-process gRPC implementation of the live-patching service used to exercise
// the check-hotfix gRPC transport without any real networking.
type mockLPSServer struct {
	lpsv1.UnimplementedLivePatchingServiceServer

	// resp and err control the RPC result.
	resp *lpsv1.GetComponentConfigResponse
	err  error

	// captured request state for assertions.
	gotComponent string
	gotToken     []string
}

func (m *mockLPSServer) GetComponentConfig(ctx context.Context, req *lpsv1.GetComponentConfigRequest) (*lpsv1.GetComponentConfigResponse, error) {
	m.gotComponent = req.GetComponentName()
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		m.gotToken = md.Get(lpsAttestedMetadataKey)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

// startMockLPS stands up the mock server on an in-memory bufconn listener and returns a dialer that
// grpcDialContext can use to reach it over a local pipe (insecure, no TLS).
func startMockLPS(t *testing.T, srv *mockLPSServer) func(context.Context, string) (net.Conn, error) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	lpsv1.RegisterLivePatchingServiceServer(s, srv)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() {
		s.Stop()
		_ = lis.Close()
	})
	return func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}
}

// newGRPCTestApp wires an App for the gRPC transport: a node config carrying the apiserver FQDN, a
// stub attested token (so no real IMDS call), and the bufconn dialer for the given mock server.
func newGRPCTestApp(t *testing.T, srv *mockLPSServer) *TestApp {
	t.Helper()
	tt := NewTestApp(t, TestAppConfig{})

	caB64 := base64.StdEncoding.EncodeToString([]byte(testCAPEM))
	nodeConfig := filepath.Join(t.TempDir(), "config.json")
	body := `{"version":"v1","api_server_config":{"api_server_name":"myapi.example.com"},"kubernetes_ca_cert":"` + caB64 + `"}`
	require.NoError(t, os.WriteFile(nodeConfig, []byte(body), 0o644))

	tt.App.nodeConfigPath = nodeConfig
	tt.App.grpcDialContext = startMockLPS(t, srv)
	tt.App.fetchAttestedToken = func(context.Context) (string, error) {
		return "attested-doc-token", nil
	}
	return tt
}

func TestFetchHotfixOverGRPC_Success(t *testing.T) {
	body := lpsPointerBody(t, map[string]string{"202604.01": "202604.01.1"})
	srv := &mockLPSServer{resp: &lpsv1.GetComponentConfigResponse{
		ComponentName: ancComponentName,
		Config:        string(body),
	}}
	tt := newGRPCTestApp(t, srv)

	got, err := tt.App.fetchHotfixOverGRPC(context.Background())
	require.NoError(t, err)
	assert.Equal(t, body, got)

	// The request must carry the ANC component name and the attested-data token in metadata.
	assert.Equal(t, ancComponentName, srv.gotComponent)
	assert.Equal(t, []string{"attested-doc-token"}, srv.gotToken)
}

func TestCheckHotfix_GRPCSuccessReadAndWrite(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	srv := &mockLPSServer{resp: &lpsv1.GetComponentConfigResponse{
		ComponentName: ancComponentName,
		Config:        string(lpsPointerBody(t, map[string]string{"202604.01": "202604.01.1"})),
	}}
	tt := newGRPCTestApp(t, srv)
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path

	outcome, err := tt.App.checkHotfix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, outcomeLPSRead, outcome)

	cfg := readStagedConfig(t, path)
	assert.Equal(t, map[string]string{"202604.01": "202604.01.1"}, cfg.Hotfixes)
}

// TestCheckHotfix_GRPCEmptyConfigIsBenign verifies that a SUCCESSFUL RPC whose config string is
// empty (proto3 default "" - a reachable LPS with nothing for this node) is a benign no-op:
// outcome noHotfixAvailable (not failed), with NO write so any existing on-disk pointer is left
// intact. This mirrors the NotFound path and avoids clobbering the cloud-init-written file.
func TestCheckHotfix_GRPCEmptyConfigIsBenign(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	srv := &mockLPSServer{resp: &lpsv1.GetComponentConfigResponse{ComponentName: ancComponentName}}
	tt := newGRPCTestApp(t, srv)
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	// Pre-seed the shared file as cloud-init would; an empty served config must not clobber it.
	require.NoError(t, os.WriteFile(path, []byte(
		`{"version":"202604.01.5","scripts_version":"202604.01.7"}`), 0644))

	outcome, err := tt.App.checkHotfix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, outcomeNoHotfixAvailable, outcome)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":"202604.01.5","scripts_version":"202604.01.7"}`, string(raw))
}

func TestCheckHotfix_GRPCBenignCodesAreNoOp(t *testing.T) {
	for name, code := range map[string]codes.Code{
		"unauthenticated":   codes.Unauthenticated,
		"permission denied": codes.PermissionDenied,
		"not found":         codes.NotFound,
	} {
		t.Run(name, func(t *testing.T) {
			srv := &mockLPSServer{err: status.Error(code, "nothing for this node")}
			tt := newGRPCTestApp(t, srv)
			path := filepath.Join(t.TempDir(), "hotfix.json")
			tt.App.hotfixVersionPath = path

			outcome, err := tt.App.checkHotfix(context.Background())
			require.NoError(t, err)
			assert.Equal(t, outcomeNoHotfixAvailable, outcome)

			// Benign no-op: nothing is staged.
			_, statErr := os.Stat(path)
			assert.True(t, os.IsNotExist(statErr), "no overlay should be staged")
		})
	}
}

func TestCheckHotfix_GRPCUnavailableFallsBackToColdStart(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	srv := &mockLPSServer{err: status.Error(codes.Unavailable, "backend down")}
	tt := newGRPCTestApp(t, srv)

	// Embed a cold-start pointer in the node config so the fallback has something to stage.
	nodeConfig := filepath.Join(t.TempDir(), "config-coldstart.json")
	caB64 := base64.StdEncoding.EncodeToString([]byte(testCAPEM))
	body := `{"version":"v1","api_server_config":{"api_server_name":"myapi.example.com"},` +
		`"kubernetes_ca_cert":"` + caB64 + `","hotfixes":{"202604.01":"202604.01.7"}}`
	require.NoError(t, os.WriteFile(nodeConfig, []byte(body), 0o644))
	tt.App.nodeConfigPath = nodeConfig

	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path

	outcome, err := tt.App.checkHotfix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, outcomeCustomDataFallback, outcome)

	cfg := readStagedConfig(t, path)
	assert.Equal(t, map[string]string{"202604.01": "202604.01.7"}, cfg.Hotfixes)
}

func TestCheckHotfix_GRPCInvalidArgumentDoesNotFallBack(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	srv := &mockLPSServer{err: status.Error(codes.InvalidArgument, "bad request")}
	tt := newGRPCTestApp(t, srv)

	// A cold-start pointer is present, but an authoritative client rejection must NOT stage it.
	nodeConfig := filepath.Join(t.TempDir(), "config-coldstart.json")
	caB64 := base64.StdEncoding.EncodeToString([]byte(testCAPEM))
	body := `{"version":"v1","api_server_config":{"api_server_name":"myapi.example.com"},` +
		`"kubernetes_ca_cert":"` + caB64 + `","hotfixes":{"202604.01":"202604.01.7"}}`
	require.NoError(t, os.WriteFile(nodeConfig, []byte(body), 0o644))
	tt.App.nodeConfigPath = nodeConfig

	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path

	outcome, err := tt.App.checkHotfix(context.Background())
	require.Error(t, err)
	assert.Equal(t, outcomeFailed, outcome)

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "authoritative rejection must not stage a cold-start overlay")
}

func TestCheckHotfix_GRPCFailOpenAlwaysExitsZero(t *testing.T) {
	srv := &mockLPSServer{err: status.Error(codes.InvalidArgument, "bad request")}
	tt := newGRPCTestApp(t, srv)
	tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "hotfix.json")

	// The cli Action must swallow the error so provisioning is never blocked.
	require.NoError(t, tt.App.runCheckHotfixCommand(context.Background()))
}

func TestMapGRPCError(t *testing.T) {
	t.Run("benign codes map to lpsUnavailable", func(t *testing.T) {
		for _, code := range []codes.Code{codes.Unauthenticated, codes.PermissionDenied, codes.NotFound} {
			mapped := mapGRPCError(status.Error(code, "x"))
			assert.True(t, isLPSUnavailable(mapped), "code %s should be benign", code)
		}
	})

	t.Run("server-side codes allow cold-start fallback", func(t *testing.T) {
		for _, code := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded, codes.Internal, codes.Unknown} {
			mapped := mapGRPCError(status.Error(code, "x"))
			assert.False(t, isLPSUnavailable(mapped))
			assert.True(t, shouldColdStartFallback(mapped), "code %s should allow fallback", code)
		}
	})

	t.Run("authoritative client codes do not fall back", func(t *testing.T) {
		for _, code := range []codes.Code{codes.InvalidArgument, codes.ResourceExhausted} {
			mapped := mapGRPCError(status.Error(code, "x"))
			assert.False(t, isLPSUnavailable(mapped))
			assert.False(t, shouldColdStartFallback(mapped), "code %s must not fall back", code)
		}
	})

	t.Run("non-status transport error falls back", func(t *testing.T) {
		mapped := mapGRPCError(context.DeadlineExceeded)
		assert.False(t, isLPSUnavailable(mapped))
		assert.True(t, shouldColdStartFallback(mapped))
	})
}

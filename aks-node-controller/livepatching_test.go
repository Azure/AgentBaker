package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	lpsv1 "github.com/Azure/agentbaker/aks-live-patching/pkg/gen/akslivepatching/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRunLivePatchingCommand_WritesConfigToStdout(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	var gotComponent string
	tt.App.livePatchingFetcher = func(_ context.Context, component string) ([]byte, error) {
		gotComponent = component
		return []byte(`{"enabled":true}`), nil
	}

	var out bytes.Buffer
	require.NoError(t, tt.App.runLivePatchingCommand(context.Background(), &out, "securityPatch"))

	// The component is passed through verbatim and the config is written with no added framing,
	// so the output can be piped straight into a JSON parser.
	assert.Equal(t, "securityPatch", gotComponent)
	assert.Equal(t, `{"enabled":true}`, out.String())
}

func TestRunLivePatchingCommand_IsGenericOverComponents(t *testing.T) {
	// The command must not pin the component the way check-hotfix does; any name the caller
	// passes reaches the service unchanged.
	for _, component := range []string{ancComponentName, "securityPatch", "localDNS", "some.new/component"} {
		t.Run(component, func(t *testing.T) {
			tt := NewTestApp(t, TestAppConfig{})
			var gotComponent string
			tt.App.livePatchingFetcher = func(_ context.Context, c string) ([]byte, error) {
				gotComponent = c
				return []byte("config-for-" + c), nil
			}

			var out bytes.Buffer
			require.NoError(t, tt.App.runLivePatchingCommand(context.Background(), &out, component))
			assert.Equal(t, component, gotComponent)
			assert.Equal(t, "config-for-"+component, out.String())
		})
	}
}

func TestRunLivePatchingCommand_EmptyConfigIsNotAnError(t *testing.T) {
	// A registered component may legitimately have an empty config published.
	tt := NewTestApp(t, TestAppConfig{})
	tt.App.livePatchingFetcher = func(context.Context, string) ([]byte, error) {
		return nil, nil
	}

	var out bytes.Buffer
	require.NoError(t, tt.App.runLivePatchingCommand(context.Background(), &out, "securityPatch"))
	assert.Empty(t, out.String())
}

func TestRunLivePatchingCommand_TrimsAndRejectsBlankComponent(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	var gotComponent string
	tt.App.livePatchingFetcher = func(_ context.Context, component string) ([]byte, error) {
		gotComponent = component
		return []byte("ok"), nil
	}

	var out bytes.Buffer
	require.NoError(t, tt.App.runLivePatchingCommand(context.Background(), &out, "  securityPatch  "))
	assert.Equal(t, "securityPatch", gotComponent)

	out.Reset()
	err := tt.App.runLivePatchingCommand(context.Background(), &out, "   ")
	require.Error(t, err)
	assert.Empty(t, out.String())
}

// TestRunLivePatchingCommand_IsNotFailOpen documents the deliberate contract difference from
// check-hotfix: fetch failures surface as errors (non-zero exit), including the codes that
// mapGRPCError folds into the benign errLPSUnavailable for the hotfix path.
func TestRunLivePatchingCommand_IsNotFailOpen(t *testing.T) {
	for name, fetchErr := range map[string]error{
		"transport failure":                   errors.New("dial failed"),
		"unavailable":                         mapGRPCError(status.Error(codes.Unavailable, "backend down")),
		"not found (benign for check-hotfix)": mapGRPCError(status.Error(codes.NotFound, "nothing published")),
		"invalid argument":                    mapGRPCError(status.Error(codes.InvalidArgument, "unknown component")),
	} {
		t.Run(name, func(t *testing.T) {
			tt := NewTestApp(t, TestAppConfig{})
			tt.App.livePatchingFetcher = func(context.Context, string) ([]byte, error) {
				return nil, fetchErr
			}

			var out bytes.Buffer
			err := tt.App.runLivePatchingCommand(context.Background(), &out, "securityPatch")
			require.Error(t, err)
			// Nothing is printed on failure, so stdout is never a partial/misleading config.
			assert.Empty(t, out.String())
		})
	}
}

func TestLivePatchingCommand_ExitCodes(t *testing.T) {
	t.Run("missing component argument", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "livepatching"})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("too many arguments", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		exitCode := tt.App.Run(context.Background(),
			[]string{"aks-node-controller", "livepatching", "securityPatch", "extra"})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("fetch failure is not fail-open", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.livePatchingFetcher = func(context.Context, string) ([]byte, error) {
			return nil, errors.New("boom")
		}
		exitCode := tt.App.Run(context.Background(),
			[]string{"aks-node-controller", "livepatching", "securityPatch"})
		assert.Equal(t, 1, exitCode)
	})

	t.Run("success", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.livePatchingFetcher = func(context.Context, string) ([]byte, error) {
			return []byte("{}"), nil
		}
		exitCode := tt.App.Run(context.Background(),
			[]string{"aks-node-controller", "livepatching", "securityPatch"})
		assert.Equal(t, 0, exitCode)
	})
}

// TestFetchComponentConfigOverGRPC_GenericComponent exercises the real gRPC transport against the
// in-process mock server, asserting the caller-supplied component reaches the wire.
func TestFetchComponentConfigOverGRPC_GenericComponent(t *testing.T) {
	srv := &mockLPSServer{resp: &lpsv1.GetComponentConfigResponse{
		ComponentName: "securityPatch",
		Config:        `{"patchLevel":"latest"}`,
	}}
	tt := newGRPCTestApp(t, srv)

	got, err := tt.App.fetchComponentConfigOverGRPC(context.Background(), "securityPatch", "livepatching")
	require.NoError(t, err)
	assert.Equal(t, `{"patchLevel":"latest"}`, string(got))
	assert.Equal(t, "securityPatch", srv.gotComponent)
	assert.Equal(t, []string{"attested-doc-token"}, srv.gotToken)
}

// TestFetchComponentConfig_DefaultsToGRPC verifies the command reaches the real transport when no
// test fetcher is injected (the mock server stands in for the service).
func TestFetchComponentConfig_DefaultsToGRPC(t *testing.T) {
	srv := &mockLPSServer{resp: &lpsv1.GetComponentConfigResponse{
		ComponentName: "localDNS",
		Config:        `{"mode":"on"}`,
	}}
	tt := newGRPCTestApp(t, srv)

	var out bytes.Buffer
	require.NoError(t, tt.App.runLivePatchingCommand(context.Background(), &out, "localDNS"))
	assert.Equal(t, `{"mode":"on"}`, out.String())
	assert.Equal(t, "localDNS", srv.gotComponent)
}

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

// testCAPEM is a self-signed CA certificate used to exercise the provision-config TLS
// trust path of the LPS gRPC dial.
const testCAPEM = `-----BEGIN CERTIFICATE-----
MIIBVDCB+6ADAgECAgEBMAoGCCqGSM49BAMCMBIxEDAOBgNVBAMTB3Rlc3QtY2Ew
HhcNMjYwNjE5MjEwNDM4WhcNMzYwNjE2MjEwNDM4WjASMRAwDgYDVQQDEwd0ZXN0
LWNhMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEDEsevoDBYiQ68iPrOeDKJLfJ
EhavIoHla/EJ5jy1EeaLp5qnDttz9IQe8PiZGSat6Dc1in8pwwQJkTcCwDMlzaNC
MEAwDgYDVR0PAQH/BAQDAgIEMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFI5z
oesQcLTRf96etb8XDK8w9wFRMAoGCCqGSM49BAMCA0gAMEUCIQCDOJZ8qJDAnEB1
2LbXQPzOc3n5Pcz3lpwQnczk/UdVJAIgcFqNv0HsWdn7Img3gNsUgSaOT1M9QBAL
52RBAH6U7DI=
-----END CERTIFICATE-----
`

// lpsPointerBody renders an LPS hotfix-pointer response body in the {"hotfixes":{...}} shape.
func lpsPointerBody(t *testing.T, hotfixes map[string]string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"hotfixes": hotfixes})
	require.NoError(t, err)
	return b
}

// readStagedConfig reads back the hotfix config check-hotfix wrote.
func readStagedConfig(t *testing.T, path string) hotfixConfig {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var cfg hotfixConfig
	require.NoError(t, json.Unmarshal(data, &cfg))
	return cfg
}

func TestParseHotfixConfig(t *testing.T) {
	t.Run("parses the hotfixes object directly", func(t *testing.T) {
		cfg, err := parseHotfixConfig([]byte(`{"hotfixes":{"202604.01":"202604.01.1","202605.01":"202605.01.2"}}`))
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"202604.01": "202604.01.1", "202605.01": "202605.01.2"}, cfg.Hotfixes)
	})

	t.Run("tolerates surrounding whitespace", func(t *testing.T) {
		cfg, err := parseHotfixConfig([]byte("  \n{\"hotfixes\":{\"202604.01\":\"202604.01.1\"}}\n "))
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"202604.01": "202604.01.1"}, cfg.Hotfixes)
	})

	t.Run("empty body is a benign no-op, not an error", func(t *testing.T) {
		// A successful RPC with an empty/unset config string means the LPS is reachable but
		// has nothing for this node - the same benign case as "{}". It must parse to a
		// zero-value config without error so the outcome stays noHotfixForBase, not failed.
		for _, body := range []string{"", "   ", "\n\t "} {
			cfg, err := parseHotfixConfig([]byte(body))
			require.NoError(t, err)
			assert.Nil(t, cfg.Hotfixes)
			assert.Equal(t, "", cfg.resolveVersion("202604.01.1"))
		}
	})

	t.Run("invalid JSON is an error", func(t *testing.T) {
		_, err := parseHotfixConfig([]byte("not json"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshaling hotfix pointer JSON")
	})

	t.Run("shares parser shape with download-hotfix readHotfixConfig", func(t *testing.T) {
		// The body served by the LPS must round-trip through the SAME shape that
		// download-hotfix's readHotfixConfig consumes.
		body := `{"hotfixes":{"202604.01":"202604.01.3"}}`
		fromLPS, err := parseHotfixConfig([]byte(body))
		require.NoError(t, err)

		path := filepath.Join(t.TempDir(), "h.json")
		require.NoError(t, os.WriteFile(path, []byte(body), 0644))
		fromFile, err := readHotfixConfig(path)
		require.NoError(t, err)
		assert.Equal(t, *fromFile, fromLPS)
	})
}

func TestCheckHotfix_SuccessReadAndWrite(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
		return lpsPointerBody(t, map[string]string{"202604.01": "202604.01.1"}), nil
	}

	outcome, err := tt.App.checkHotfix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, outcomeLPSRead, outcome)

	cfg := readStagedConfig(t, path)
	assert.Equal(t, map[string]string{"202604.01": "202604.01.1"}, cfg.Hotfixes)
}

func TestCheckHotfix_NoHotfixForBase(t *testing.T) {
	origVersion := Version
	Version = "202607.15.0" // base not present in the LPS pointer
	defer func() { Version = origVersion }()

	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
		return lpsPointerBody(t, map[string]string{"202604.01": "202604.01.1"}), nil
	}

	outcome, err := tt.App.checkHotfix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, outcomeNoHotfixForBase, outcome)

	// The full pointer is still staged so download-hotfix re-resolves authoritatively.
	cfg := readStagedConfig(t, path)
	assert.Equal(t, map[string]string{"202604.01": "202604.01.1"}, cfg.Hotfixes)
}

// TestCheckHotfix_EmptyServedConfigIsBenignAndPreservesFile guards the no-clobber contract:
// the on-disk pointer is the SAME file cloud-init populates with "version"/"scripts_version".
// A reachable LPS that serves an empty hotfixes map (a "{}" body, or a legacy-only body whose
// Version we intentionally drop) must be treated exactly like a benign NotFound - no write, so
// the cloud-init-written version/scripts_version survive. Writing an empty map here would
// disable the existing cloud-init provisioning-hotfix path.
func TestCheckHotfix_EmptyServedConfigIsBenignAndPreservesFile(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	cloudInitFile := `{"version":"202604.01.5","scripts_version":"202604.01.7","hotfixes":{"202604.01":"202604.01.5"}}`
	cases := map[string]string{
		"empty object":     `{}`,
		"empty map":        `{"hotfixes":{}}`,
		"legacy-only body": `{"version":"202604.01.9"}`,
		"empty body":       ``,
	}
	for name, servedBody := range cases {
		t.Run(name, func(t *testing.T) {
			tt := NewTestApp(t, TestAppConfig{})
			path := filepath.Join(t.TempDir(), "hotfix.json")
			tt.App.hotfixVersionPath = path
			// Pre-seed the shared file exactly as cloud-init would.
			require.NoError(t, os.WriteFile(path, []byte(cloudInitFile), 0644))

			body := servedBody
			tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
				return []byte(body), nil
			}

			outcome, err := tt.App.checkHotfix(context.Background())
			require.NoError(t, err)
			assert.Equal(t, outcomeNoHotfixAvailable, outcome)

			// The cloud-init file must be left byte-for-byte intact.
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.JSONEq(t, cloudInitFile, string(raw))
		})
	}
}

// TestCheckHotfix_PreservesVersionAndScriptsVersionOnWrite guards the "sharper half": even for
// a NON-empty served hotfixes map (a real hotfix rollout), the write must be a read-modify-write
// that preserves the cloud-init-written version/scripts_version and replaces only the hotfixes
// map. The LPS payload is hotfixes-only by design, so rewriting the whole file naively would
// drop scripts_version and silently disable the CSE-scripts hotfix.
func TestCheckHotfix_PreservesVersionAndScriptsVersionOnWrite(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	// cloud-init seeds version + scripts_version + a stale hotfixes map.
	require.NoError(t, os.WriteFile(path, []byte(
		`{"version":"202604.01.5","scripts_version":"202604.01.7","hotfixes":{"202604.01":"202604.01.5"}}`), 0644))

	tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
		return lpsPointerBody(t, map[string]string{"202604.01": "202604.01.9"}), nil
	}

	outcome, err := tt.App.checkHotfix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, outcomeLPSRead, outcome)

	cfg := readStagedConfig(t, path)
	// hotfixes map replaced by the LPS-served one...
	assert.Equal(t, map[string]string{"202604.01": "202604.01.9"}, cfg.Hotfixes)
	// ...but version/scripts_version preserved from the cloud-init file.
	assert.Equal(t, "202604.01.5", cfg.Version)
	assert.Equal(t, "202604.01.7", cfg.ScriptsVersion)
}

func TestCheckHotfix_LPSUnavailableIsBenign(t *testing.T) {
	// A reachable LPS that has no hotfix published for this node is the expected steady state.
	// It must be a benign no-op: outcome noHotfixAvailable, no error, nothing staged, and NO
	// cold-start overlay even when the node config carries an embedded pointer. The benign
	// signal is the errLPSUnavailable sentinel; which gRPC codes map to it is covered by the
	// gRPC transport tests. Both the bare sentinel and a code-wrapped form must classify as
	// benign (errors.Is through the wrap).
	fetchErrs := map[string]error{
		"bare sentinel":     errLPSUnavailable,
		"wrapped with code": fmt.Errorf("%w (code %s)", errLPSUnavailable, codes.NotFound),
	}
	for name, fetchErr := range fetchErrs {
		t.Run(name, func(t *testing.T) {
			tt := NewTestApp(t, TestAppConfig{})
			path := filepath.Join(t.TempDir(), "hotfix.json")
			tt.App.hotfixVersionPath = path

			// Even with a cold-start pointer present, a benign LPS answer stages no overlay.
			nodeConfig := filepath.Join(t.TempDir(), "aks-node-controller-config.json")
			require.NoError(t, os.WriteFile(nodeConfig, []byte(
				`{"version":"v1","hotfixes":{"202604.01":"202604.01.9"}}`), 0644))
			tt.App.nodeConfigPath = nodeConfig

			tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
				return nil, fetchErr
			}

			outcome, err := tt.App.checkHotfix(context.Background())
			require.NoError(t, err)
			assert.Equal(t, outcomeNoHotfixAvailable, outcome)

			// No overlay staged.
			_, statErr := os.Stat(path)
			assert.True(t, os.IsNotExist(statErr))
		})
	}
}

func TestCheckHotfix_FetchErrorFailsOpenWithoutFallback(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	// No node config -> no cold-start fallback available.
	tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nonexistent-config.json")

	// Transport-level failures (not benign 401/403/404) with no fallback -> failed.
	cases := map[string]error{
		"timeout":        context.DeadlineExceeded,
		"connection err": errors.New("dial tcp: connection refused"),
		"server error":   errors.New("LPS returned status 500"),
	}
	for name, fetchErr := range cases {
		t.Run(name, func(t *testing.T) {
			tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
				return nil, fetchErr
			}
			outcome, err := tt.App.checkHotfix(context.Background())
			assert.Equal(t, outcomeFailed, outcome)
			assert.Error(t, err)
			// Nothing should be staged.
			_, statErr := os.Stat(path)
			assert.True(t, os.IsNotExist(statErr))
		})
	}
}

func TestCheckHotfix_InvalidPointerFailsOpen(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
		return []byte("not valid json"), nil
	}

	outcome, err := tt.App.checkHotfix(context.Background())
	assert.Equal(t, outcomeFailed, outcome)
	assert.Error(t, err)
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr))
}

func TestCheckHotfix_ColdStartFallback(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path

	// Node config carries a lenient top-level hotfixes pointer (PoC cold-start contract).
	nodeConfig := filepath.Join(t.TempDir(), "aks-node-controller-config.json")
	require.NoError(t, os.WriteFile(nodeConfig, []byte(
		`{"version":"v1","hotfixes":{"202604.01":"202604.01.2"}}`), 0644))
	tt.App.nodeConfigPath = nodeConfig

	tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
		return nil, errors.New("dial tcp: connection refused")
	}

	outcome, err := tt.App.checkHotfix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, outcomeCustomDataFallback, outcome)

	cfg := readStagedConfig(t, path)
	assert.Equal(t, map[string]string{"202604.01": "202604.01.2"}, cfg.Hotfixes)
}

func TestCheckHotfix_ColdStartNoPointerFails(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path

	nodeConfig := filepath.Join(t.TempDir(), "aks-node-controller-config.json")
	require.NoError(t, os.WriteFile(nodeConfig, []byte(`{"version":"v1"}`), 0644))
	tt.App.nodeConfigPath = nodeConfig
	tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
		return nil, errors.New("dial tcp: connection refused")
	}

	outcome, err := tt.App.checkHotfix(context.Background())
	assert.Equal(t, outcomeFailed, outcome)
	assert.Error(t, err)
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr))
}

// TestCheckHotfix_FallbackOnlyForUnreachableLPS verifies that the cold-start fallback is used
// only when the LPS could not be reached or is server-broken (transport error / 5xx). A
// reachable LPS returning a non-benign 4xx (e.g. 400/429) is authoritative: check-hotfix must
// NOT stage the (possibly stale) cold-start pointer even though one is present.
func TestCheckHotfix_FallbackOnlyForUnreachableLPS(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	cases := []struct {
		name        string
		fetchErr    error
		wantOutcome checkHotfixOutcome
		wantStaged  bool
	}{
		{"server Unavailable falls back to cold-start", &lpsGRPCStatusError{code: codes.Unavailable, fallbackAllowed: true}, outcomeCustomDataFallback, true},
		{"transport error falls back to cold-start", errors.New("dial tcp: connection refused"), outcomeCustomDataFallback, true},
		{"ResourceExhausted does not fall back", &lpsGRPCStatusError{code: codes.ResourceExhausted, fallbackAllowed: false}, outcomeFailed, false},
		{"InvalidArgument does not fall back", &lpsGRPCStatusError{code: codes.InvalidArgument, fallbackAllowed: false}, outcomeFailed, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tt := NewTestApp(t, TestAppConfig{})
			path := filepath.Join(t.TempDir(), "hotfix.json")
			tt.App.hotfixVersionPath = path

			// A cold-start pointer IS present, so the outcome hinges purely on whether the
			// fetch error is treated as "unreachable" (fall back) or "reachable client error".
			nodeConfig := filepath.Join(t.TempDir(), "aks-node-controller-config.json")
			require.NoError(t, os.WriteFile(nodeConfig, []byte(
				`{"version":"v1","hotfixes":{"202604.01":"202604.01.2"}}`), 0644))
			tt.App.nodeConfigPath = nodeConfig
			tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
				return nil, tc.fetchErr
			}

			outcome, _ := tt.App.checkHotfix(context.Background())
			assert.Equal(t, tc.wantOutcome, outcome)
			_, statErr := os.Stat(path)
			if tc.wantStaged {
				assert.NoError(t, statErr, "expected the cold-start pointer to be staged")
			} else {
				assert.True(t, os.IsNotExist(statErr), "expected nothing to be staged")
			}
		})
	}
}

// TestRunCheckHotfixCommand_AlwaysFailOpen verifies the cli Action always returns nil
// (exit 0) and emits telemetry, regardless of the underlying outcome.
func TestRunCheckHotfixCommand_AlwaysFailOpen(t *testing.T) {
	t.Run("success path emits informational event and exits 0", func(t *testing.T) {
		origVersion := Version
		Version = "202604.01.0"
		defer func() { Version = origVersion }()

		tt := NewTestApp(t, TestAppConfig{})
		tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "hotfix.json")
		tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
			return lpsPointerBody(t, map[string]string{"202604.01": "202604.01.1"}), nil
		}

		err := tt.App.runCheckHotfixCommand(context.Background())
		require.NoError(t, err)

		events := tt.eventLogger.Events()
		require.Len(t, events, 1)
		assert.Equal(t, "AKS.AKSNodeController.CheckHotfix", events[0].TaskName)
		assert.Equal(t, "Informational", events[0].EventLevel)
		assert.Contains(t, events[0].Message, string(outcomeLPSRead))
	})

	t.Run("failure path emits error event but still exits 0", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "hotfix.json")
		tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nonexistent.json")
		tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
			return nil, errors.New("LPS returned status 500")
		}

		err := tt.App.runCheckHotfixCommand(context.Background())
		require.NoError(t, err)

		events := tt.eventLogger.Events()
		require.Len(t, events, 1)
		assert.Equal(t, "AKS.AKSNodeController.CheckHotfix", events[0].TaskName)
		assert.Equal(t, "Error", events[0].EventLevel)
		assert.Contains(t, events[0].Message, string(outcomeFailed))
	})

	t.Run("cli wiring returns exit code 0 even on fetch failure", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "hotfix.json")
		tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nonexistent.json")
		tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
			return nil, errors.New("boom")
		}
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "check-hotfix"})
		assert.Equal(t, 0, exitCode)
	})

	t.Run("a panic in the workflow is recovered and still exits 0", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "hotfix.json")
		// A fetcher that panics simulates an unexpected crash anywhere in the workflow.
		tt.App.checkHotfixFetcher = func(context.Context) ([]byte, error) {
			panic("unexpected boom")
		}

		err := tt.App.runCheckHotfixCommand(context.Background())
		require.NoError(t, err, "a panic must be swallowed so provisioning proceeds")

		events := tt.eventLogger.Events()
		require.Len(t, events, 1)
		assert.Equal(t, "AKS.AKSNodeController.CheckHotfix", events[0].TaskName)
		assert.Equal(t, "Error", events[0].EventLevel)
		assert.Contains(t, events[0].Message, string(outcomeFailed))
		assert.Contains(t, events[0].Message, "panic")
	})
}

func TestCheckHotfix_DefaultsToLPSFetcherWhenNoInjection(t *testing.T) {
	// With no injected fetcher and no readable node config, the real path is exercised: it
	// must fail-open. Point the node-config source at a nonexistent path so LPS endpoint
	// resolution fails deterministically and the network is never actually dialed.
	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nonexistent.json")
	// checkHotfixFetcher intentionally nil.

	err := tt.App.runCheckHotfixCommand(context.Background())
	require.NoError(t, err)
}

func TestAttestedToken_InjectionOverridesIMDS(t *testing.T) {
	t.Run("injected token is returned without networking", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.fetchAttestedToken = func(context.Context) (string, error) {
			return "injected-signature", nil
		}
		tok, err := tt.App.attestedToken(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "injected-signature", tok)
	})

	t.Run("injected error propagates", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.fetchAttestedToken = func(context.Context) (string, error) {
			return "", errors.New("imds down")
		}
		_, err := tt.App.attestedToken(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "imds down")
	})
}

func TestLPSTargetFromNodeConfig(t *testing.T) {
	// A minimal AKSNodeConfig in the on-disk shape: MarshalConfigurationV1 sets
	// UseProtoNames=true, so production JSON uses proto (snake_case) field names.
	caPEM := "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"
	caB64 := base64.StdEncoding.EncodeToString([]byte(caPEM))

	t.Run("reads fqdn and decodes CA", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		p := filepath.Join(t.TempDir(), "config.json")
		body := `{"version":"v1","api_server_config":{"api_server_name":"myapi.example.com"},"kubernetes_ca_cert":"` + caB64 + `"}`
		require.NoError(t, os.WriteFile(p, []byte(body), 0644))
		tt.App.nodeConfigPath = p
		tt.App.nbcCmdPath = filepath.Join(t.TempDir(), "must-not-be-read.sh")

		fqdn, ca, err := tt.App.lpsTargetFromNodeConfig()
		require.NoError(t, err)
		assert.Equal(t, "myapi.example.com", fqdn)
		assert.Equal(t, []byte(caPEM), ca)
	})

	t.Run("missing apiserver name is an error", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		p := filepath.Join(t.TempDir(), "config.json")
		require.NoError(t, os.WriteFile(p, []byte(`{"version":"v1"}`), 0644))
		tt.App.nodeConfigPath = p

		_, _, err := tt.App.lpsTargetFromNodeConfig()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "api_server_name")
	})

	t.Run("missing config.json falls back to nbc-cmd.sh", func(t *testing.T) {
		dir := t.TempDir()
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.nodeConfigPath = filepath.Join(dir, "nope.json")

		caPEM := "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"
		caB64 := base64.StdEncoding.EncodeToString([]byte(caPEM))
		cmdPath := filepath.Join(dir, "nbc-cmd.sh")
		cmdContent := fmt.Sprintf(
			`PROVISION_OUTPUT="/tmp/out"; API_SERVER_NAME=fallback.example.com `+
				`KUBE_CA_CRT="%s" /usr/bin/nohup /bin/bash -c "/bin/bash /opt/azure/containers/provision_start.sh"`,
			caB64)
		require.NoError(t, os.WriteFile(cmdPath, []byte(cmdContent), 0600))
		tt.App.nbcCmdPath = cmdPath

		fqdn, ca, err := tt.App.lpsTargetFromNodeConfig()
		require.NoError(t, err)
		assert.Equal(t, "fallback.example.com", fqdn)
		assert.Equal(t, []byte(caPEM), ca)
	})

	t.Run("missing config.json and missing nbc-cmd.sh is an error", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nope.json")
		tt.App.nbcCmdPath = filepath.Join(t.TempDir(), "also-nope.sh")
		_, _, err := tt.App.lpsTargetFromNodeConfig()
		require.Error(t, err)
	})
}

func TestLPSTargetFromNBCCmd(t *testing.T) {
	caPEM := "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"
	caB64 := base64.StdEncoding.EncodeToString([]byte(caPEM))

	t.Run("reads fqdn and decodes CA", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		p := filepath.Join(t.TempDir(), "nbc-cmd.sh")
		content := fmt.Sprintf(`API_SERVER_NAME=myapi.example.com KUBE_CA_CRT="%s" /usr/bin/nohup /bin/bash -c "provision"`, caB64)
		require.NoError(t, os.WriteFile(p, []byte(content), 0600))
		tt.App.nbcCmdPath = p

		fqdn, ca, err := tt.App.lpsTargetFromNBCCmd()
		require.NoError(t, err)
		assert.Equal(t, "myapi.example.com", fqdn)
		assert.Equal(t, []byte(caPEM), ca)
	})

	t.Run("missing API_SERVER_NAME is an error", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		p := filepath.Join(t.TempDir(), "nbc-cmd.sh")
		content := fmt.Sprintf(`KUBE_CA_CRT="%s" /usr/bin/nohup /bin/bash -c "provision"`, caB64)
		require.NoError(t, os.WriteFile(p, []byte(content), 0600))
		tt.App.nbcCmdPath = p

		_, _, err := tt.App.lpsTargetFromNBCCmd()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "API_SERVER_NAME")
	})

	t.Run("missing file is an error", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.nbcCmdPath = filepath.Join(t.TempDir(), "nope.sh")
		_, _, err := tt.App.lpsTargetFromNBCCmd()
		require.Error(t, err)
	})

	t.Run("CA is optional", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		p := filepath.Join(t.TempDir(), "nbc-cmd.sh")
		require.NoError(t, os.WriteFile(p, []byte(`API_SERVER_NAME=myapi.example.com /usr/bin/nohup /bin/bash -c "provision"`), 0600))
		tt.App.nbcCmdPath = p

		fqdn, ca, err := tt.App.lpsTargetFromNBCCmd()
		require.NoError(t, err)
		assert.Equal(t, "myapi.example.com", fqdn)
		assert.Nil(t, ca)
	})

	t.Run("invalid CA is an error", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		p := filepath.Join(t.TempDir(), "nbc-cmd.sh")
		require.NoError(t, os.WriteFile(p, []byte(`API_SERVER_NAME=myapi.example.com KUBE_CA_CRT="not-base64" /usr/bin/nohup /bin/bash -c "provision"`), 0600))
		tt.App.nbcCmdPath = p

		_, _, err := tt.App.lpsTargetFromNBCCmd()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "KUBE_CA_CRT")
	})
}

func TestColdStartHotfixConfig(t *testing.T) {
	t.Run("missing file returns not-present without error", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nope.json")
		cfg, ok, err := tt.App.coldStartHotfixConfig()
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Nil(t, cfg.Hotfixes)
	})

	t.Run("present pointer is parsed", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		p := filepath.Join(t.TempDir(), "config.json")
		require.NoError(t, os.WriteFile(p, []byte(`{"version":"v1","hotfixes":{"202604.01":"202604.01.5"}}`), 0644))
		tt.App.nodeConfigPath = p
		cfg, ok, err := tt.App.coldStartHotfixConfig()
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, map[string]string{"202604.01": "202604.01.5"}, cfg.Hotfixes)
	})

	t.Run("no hotfixes key returns not-present", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		p := filepath.Join(t.TempDir(), "config.json")
		require.NoError(t, os.WriteFile(p, []byte(`{"version":"v1"}`), 0644))
		tt.App.nodeConfigPath = p
		_, ok, err := tt.App.coldStartHotfixConfig()
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestWriteHotfixConfig_ShapeAndAtomicity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hotfix.json")
	require.NoError(t, writeHotfixConfig(path, hotfixConfig{Hotfixes: map[string]string{"202604.01": "202604.01.1"}}))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	// Must serialize in the {"hotfixes":{...}} shape with no legacy version field.
	assert.JSONEq(t, `{"hotfixes":{"202604.01":"202604.01.1"}}`, string(raw))

	// Round-trips through download-hotfix's reader.
	cfg, err := readHotfixConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "202604.01.1", cfg.resolveVersion("202604.01.0"))
}

// TestWriteHotfixConfig_EmptyMapKeepsStableKey guards the on-disk/LPS contract: even when the
// map is empty or nil, the staged file must retain a top-level "hotfixes" key ({"hotfixes":{}})
// rather than collapsing to {} (which hotfixConfig's omitempty tag would otherwise produce).
func TestWriteHotfixConfig_EmptyMapKeepsStableKey(t *testing.T) {
	cases := map[string]hotfixConfig{
		"empty map": {Hotfixes: map[string]string{}},
		"nil map":   {Hotfixes: nil},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hotfix.json")
			require.NoError(t, writeHotfixConfig(path, cfg))

			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.JSONEq(t, `{"hotfixes":{}}`, string(raw))
		})
	}
}

// TestWriteHotfixConfig_PreservesVersionsAndDropsArtifacts guards the read-modify-write:
// version fields remain compatible with cloud-init, while the retired artifacts contract is removed.
func TestWriteHotfixConfig_PreservesVersionsAndDropsArtifacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hotfix.json")
	existing := `{"version":"202604.01.5","scripts_version":"202604.01.7",` +
		`"hotfixes":{"202604.01":"202604.01.5"},` +
		`"artifacts":{"202604.01.5":{"linux-ubuntu-22.04-amd64":` +
		`{"url":"https://packages.microsoft.com/fake.deb","sha256":"abc123"}}}}`
	require.NoError(t, os.WriteFile(path, []byte(existing), 0644))

	require.NoError(t, writeHotfixConfig(path, hotfixConfig{Hotfixes: map[string]string{"202604.01": "202604.01.9"}}))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":"202604.01.5","scripts_version":"202604.01.7","hotfixes":{"202604.01":"202604.01.9"}}`, string(raw))
}

func TestWriteHotfixConfig_FileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes are not represented on windows")
	}
	cfg := hotfixConfig{Hotfixes: map[string]string{"202604.01": "202604.01.1"}}

	t.Run("new file uses the 0644 cloud-init contract (not CreateTemp's 0600)", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix.json")
		require.NoError(t, writeHotfixConfig(path, cfg))
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
	})

	t.Run("existing file mode is preserved", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hotfix.json")
		require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))
		require.NoError(t, writeHotfixConfig(path, cfg))
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})
}

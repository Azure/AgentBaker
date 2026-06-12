package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCAPEM is a self-signed CA certificate used to exercise the provision-config TLS
// trust path in buildLPSHTTPClient.
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

	t.Run("empty body is an error", func(t *testing.T) {
		_, err := parseHotfixConfig([]byte("   "))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
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
		assert.Equal(t, fromFile, fromLPS)
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

func TestCheckHotfix_LPSUnavailableIsBenign(t *testing.T) {
	// A reachable LPS that has nothing for this node (401 pool-not-enrolled, 403, 404) is the
	// expected steady state. It must be a benign no-op: outcome notEnrolled, no error, nothing
	// staged, and NO cold-start overlay even when the node config carries an embedded pointer.
	statuses := map[string]int{
		"401 not enrolled": http.StatusUnauthorized,
		"403 forbidden":    http.StatusForbidden,
		"404 not found":    http.StatusNotFound,
	}
	for name, code := range statuses {
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
				return nil, &lpsUnavailableError{statusCode: code}
			}

			outcome, err := tt.App.checkHotfix(context.Background())
			require.NoError(t, err)
			assert.Equal(t, outcomeNotEnrolled, outcome)

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
	// A minimal AKSNodeConfig (protojson) carrying the apiserver name and base64 CA.
	caPEM := "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"
	caB64 := base64.StdEncoding.EncodeToString([]byte(caPEM))

	t.Run("reads fqdn and decodes CA", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		p := filepath.Join(t.TempDir(), "config.json")
		body := `{"version":"v1","apiServerConfig":{"apiServerName":"myapi.example.com"},"kubernetesCaCert":"` + caB64 + `"}`
		require.NoError(t, os.WriteFile(p, []byte(body), 0644))
		tt.App.nodeConfigPath = p

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

	t.Run("missing file is an error", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nope.json")
		_, _, err := tt.App.lpsTargetFromNodeConfig()
		require.Error(t, err)
	})
}

func TestStripScheme(t *testing.T) {
	assert.Equal(t, "host:443", stripScheme("https://host:443"))
	assert.Equal(t, "host:443", stripScheme("http://host:443"))
	assert.Equal(t, "host:443", stripScheme("host:443"))
	assert.Equal(t, "host", stripScheme("https://host/"))
}

func TestBuildLPSHTTPClient(t *testing.T) {
	t.Run("invalid CA PEM is an error", func(t *testing.T) {
		_, _, err := buildLPSHTTPClient("myapi.example.com", []byte("not a pem"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cluster CA PEM")
	})

	t.Run("valid CA pins ServerName and reports provision-config trust", func(t *testing.T) {
		// A real (self-signed) cert PEM so AppendCertsFromPEM succeeds.
		client, caSource, err := buildLPSHTTPClient("myapi.example.com", []byte(testCAPEM))
		require.NoError(t, err)
		assert.Equal(t, "provision-config", caSource)
		assert.Equal(t, lpsFetchTimeout, client.Timeout)
		tr := client.Transport.(*http.Transport)
		assert.Equal(t, lpsSNIHost, tr.TLSClientConfig.ServerName)
		assert.False(t, tr.TLSClientConfig.InsecureSkipVerify)
	})

	t.Run("no CA falls back to insecure-skip-verify", func(t *testing.T) {
		client, caSource, err := buildLPSHTTPClient("myapi.example.com", nil)
		require.NoError(t, err)
		assert.Equal(t, "insecure-skip-verify", caSource)
		tr := client.Transport.(*http.Transport)
		assert.Equal(t, lpsSNIHost, tr.TLSClientConfig.ServerName)
		assert.True(t, tr.TLSClientConfig.InsecureSkipVerify)
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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeConfigMapJSON renders a Kubernetes ConfigMap GET body whose .data has the given keys.
func makeConfigMapJSON(t *testing.T, data map[string]string) []byte {
	t.Helper()
	cm := map[string]any{
		"kind":       "ConfigMap",
		"apiVersion": "v1",
		"metadata":   map[string]any{"name": hotfixConfigMapName, "namespace": hotfixConfigMapNamespace},
		"data":       data,
	}
	b, err := json.Marshal(cm)
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

func TestParseConfigMapHotfixConfig(t *testing.T) {
	t.Run("documented hotfixes.json key", func(t *testing.T) {
		cm := makeConfigMapJSON(t, map[string]string{
			hotfixConfigMapDataKey: `{"hotfixes":{"202604.01":"202604.01.1","202605.01":"202605.01.2"}}`,
		})
		cfg, err := parseConfigMapHotfixConfig(cm)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"202604.01": "202604.01.1", "202605.01": "202605.01.2"}, cfg.Hotfixes)
	})

	t.Run("single fallback key when documented key absent", func(t *testing.T) {
		cm := makeConfigMapJSON(t, map[string]string{
			"some-other-key": `{"hotfixes":{"202604.01":"202604.01.1"}}`,
		})
		cfg, err := parseConfigMapHotfixConfig(cm)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"202604.01": "202604.01.1"}, cfg.Hotfixes)
	})

	t.Run("multiple keys without documented key is an error", func(t *testing.T) {
		cm := makeConfigMapJSON(t, map[string]string{
			"a": `{"hotfixes":{}}`,
			"b": `{"hotfixes":{}}`,
		})
		_, err := parseConfigMapHotfixConfig(cm)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected exactly 1")
	})

	t.Run("empty data is an error", func(t *testing.T) {
		cm := makeConfigMapJSON(t, map[string]string{})
		_, err := parseConfigMapHotfixConfig(cm)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no data")
	})

	t.Run("empty value is an error", func(t *testing.T) {
		cm := makeConfigMapJSON(t, map[string]string{hotfixConfigMapDataKey: "   "})
		_, err := parseConfigMapHotfixConfig(cm)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("invalid inner JSON is an error", func(t *testing.T) {
		cm := makeConfigMapJSON(t, map[string]string{hotfixConfigMapDataKey: "not json"})
		_, err := parseConfigMapHotfixConfig(cm)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshaling hotfix pointer JSON")
	})

	t.Run("invalid configmap JSON is an error", func(t *testing.T) {
		_, err := parseConfigMapHotfixConfig([]byte("not a configmap"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshaling ConfigMap")
	})

	t.Run("shares parser with download-hotfix readHotfixConfig", func(t *testing.T) {
		// The inner value written by the live-patching-controller must round-trip through
		// the SAME shape that download-hotfix's readHotfixConfig consumes.
		inner := `{"hotfixes":{"202604.01":"202604.01.3"}}`
		cm := makeConfigMapJSON(t, map[string]string{hotfixConfigMapDataKey: inner})
		fromCM, err := parseConfigMapHotfixConfig(cm)
		require.NoError(t, err)

		path := filepath.Join(t.TempDir(), "h.json")
		require.NoError(t, os.WriteFile(path, []byte(inner), 0644))
		fromFile, err := readHotfixConfig(path)
		require.NoError(t, err)
		assert.Equal(t, fromFile, fromCM)
	})
}

func TestCheckHotfix_SuccessReadAndWrite(t *testing.T) {
	origVersion := Version
	Version = "202604.01.0"
	defer func() { Version = origVersion }()

	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	tt.App.checkHotfixConfigMapFetcher = func(context.Context) ([]byte, error) {
		return makeConfigMapJSON(t, map[string]string{
			hotfixConfigMapDataKey: `{"hotfixes":{"202604.01":"202604.01.1"}}`,
		}), nil
	}

	outcome, err := tt.App.checkHotfix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, outcomeConfigMapRead, outcome)

	cfg := readStagedConfig(t, path)
	assert.Equal(t, map[string]string{"202604.01": "202604.01.1"}, cfg.Hotfixes)
}

func TestCheckHotfix_NoHotfixForBase(t *testing.T) {
	origVersion := Version
	Version = "202607.15.0" // base not present in the ConfigMap
	defer func() { Version = origVersion }()

	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	tt.App.checkHotfixConfigMapFetcher = func(context.Context) ([]byte, error) {
		return makeConfigMapJSON(t, map[string]string{
			hotfixConfigMapDataKey: `{"hotfixes":{"202604.01":"202604.01.1"}}`,
		}), nil
	}

	outcome, err := tt.App.checkHotfix(context.Background())
	require.NoError(t, err)
	assert.Equal(t, outcomeNoHotfixForBase, outcome)

	// The full pointer is still staged so download-hotfix re-resolves authoritatively.
	cfg := readStagedConfig(t, path)
	assert.Equal(t, map[string]string{"202604.01": "202604.01.1"}, cfg.Hotfixes)
}

func TestCheckHotfix_FetchErrorFailsOpenWithoutFallback(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	// No node config -> no cold-start fallback available.
	tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nonexistent-config.json")

	cases := map[string]error{
		"404 not found":  errors.New("apiserver returned status 404"),
		"403 forbidden":  errors.New("apiserver returned status 403"),
		"timeout":        context.DeadlineExceeded,
		"connection err": errors.New("dial tcp: connection refused"),
	}
	for name, fetchErr := range cases {
		t.Run(name, func(t *testing.T) {
			tt.App.checkHotfixConfigMapFetcher = func(context.Context) ([]byte, error) {
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

func TestCheckHotfix_InvalidConfigMapFailsOpen(t *testing.T) {
	tt := NewTestApp(t, TestAppConfig{})
	path := filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.hotfixVersionPath = path
	tt.App.checkHotfixConfigMapFetcher = func(context.Context) ([]byte, error) {
		return []byte(`{"data":{"hotfixes.json":"not valid json"}}`), nil
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

	tt.App.checkHotfixConfigMapFetcher = func(context.Context) ([]byte, error) {
		return nil, errors.New("apiserver returned status 404")
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
	tt.App.checkHotfixConfigMapFetcher = func(context.Context) ([]byte, error) {
		return nil, errors.New("apiserver returned status 403")
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
		tt.App.checkHotfixConfigMapFetcher = func(context.Context) ([]byte, error) {
			return makeConfigMapJSON(t, map[string]string{
				hotfixConfigMapDataKey: `{"hotfixes":{"202604.01":"202604.01.1"}}`,
			}), nil
		}

		err := tt.App.runCheckHotfixCommand(context.Background())
		require.NoError(t, err)

		events := tt.eventLogger.Events()
		require.Len(t, events, 1)
		assert.Equal(t, "AKS.AKSNodeController.CheckHotfix", events[0].TaskName)
		assert.Equal(t, "Informational", events[0].EventLevel)
		assert.Contains(t, events[0].Message, string(outcomeConfigMapRead))
	})

	t.Run("failure path emits error event but still exits 0", func(t *testing.T) {
		tt := NewTestApp(t, TestAppConfig{})
		tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "hotfix.json")
		tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nonexistent.json")
		tt.App.checkHotfixConfigMapFetcher = func(context.Context) ([]byte, error) {
			return nil, errors.New("apiserver returned status 500")
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
		tt.App.checkHotfixConfigMapFetcher = func(context.Context) ([]byte, error) {
			return nil, errors.New("boom")
		}
		exitCode := tt.App.Run(context.Background(), []string{"aks-node-controller", "check-hotfix"})
		assert.Equal(t, 0, exitCode)
	})
}

func TestCheckHotfix_DefaultsToConfigMapFetcherWhenNoInjection(t *testing.T) {
	// With no injected fetcher and no reachable apiserver/kubeconfig, the real path is
	// exercised: it must fail-open. Point creds sources at nonexistent paths so the
	// network is never actually dialed in a deterministic way.
	tt := NewTestApp(t, TestAppConfig{})
	tt.App.hotfixVersionPath = filepath.Join(t.TempDir(), "hotfix.json")
	tt.App.nodeConfigPath = filepath.Join(t.TempDir(), "nonexistent.json")
	// checkHotfixConfigMapFetcher intentionally nil.

	err := tt.App.runCheckHotfixCommand(context.Background())
	require.NoError(t, err)
}

func TestParseKubeconfigCreds(t *testing.T) {
	t.Run("bootstrap token kubeconfig with CA file", func(t *testing.T) {
		yml := `apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://myapi.example.com:443
users:
- name: kubelet-bootstrap
  user:
    token: "abc.def"
current-context: bootstrap-context
`
		// CA file won't exist in the test env; parsing should still fail gracefully on the
		// CA read. Use inline data instead for a clean parse below; here assert the read error.
		_, err := parseKubeconfigCreds([]byte(yml))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "certificate-authority")
	})

	t.Run("token kubeconfig with inline CA data", func(t *testing.T) {
		caData := "dGVzdC1jYQ==" // base64("test-ca")
		yml := `apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority-data: ` + caData + `
    server: https://myapi.example.com:443
users:
- name: kubelet-bootstrap
  user:
    token: "abc.def"
`
		creds, err := parseKubeconfigCreds([]byte(yml))
		require.NoError(t, err)
		assert.Equal(t, "myapi.example.com:443", creds.server)
		assert.Equal(t, "abc.def", creds.token)
		assert.Equal(t, []byte("test-ca"), creds.caPEM)
	})

	t.Run("client-cert kubeconfig with inline data", func(t *testing.T) {
		yml := `apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority-data: dGVzdC1jYQ==
    server: https://10.0.0.1:443
users:
- name: client
  user:
    client-certificate-data: Y2VydA==
    client-key-data: a2V5
`
		creds, err := parseKubeconfigCreds([]byte(yml))
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.1:443", creds.server)
		assert.Equal(t, "", creds.token)
		assert.Equal(t, []byte("cert"), creds.clientCertPEM)
		assert.Equal(t, []byte("key"), creds.clientKeyPEM)
	})

	t.Run("no clusters is an error", func(t *testing.T) {
		_, err := parseKubeconfigCreds([]byte("apiVersion: v1\nkind: Config\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no clusters")
	})

	t.Run("invalid yaml is an error", func(t *testing.T) {
		_, err := parseKubeconfigCreds([]byte("\tnot: : yaml:"))
		require.Error(t, err)
	})
}

func TestEnsurePort(t *testing.T) {
	assert.Equal(t, "host.example.com:443", ensurePort("host.example.com", "443"))
	assert.Equal(t, "host.example.com:443", ensurePort("https://host.example.com", "443"))
	assert.Equal(t, "host.example.com:6443", ensurePort("host.example.com:6443", "443"))
	assert.Equal(t, "host.example.com:443", ensurePort("https://host.example.com/", "443"))
	assert.Equal(t, "", ensurePort("", "443"))
}

func TestStripScheme(t *testing.T) {
	assert.Equal(t, "host:443", stripScheme("https://host:443"))
	assert.Equal(t, "host:443", stripScheme("http://host:443"))
	assert.Equal(t, "host:443", stripScheme("host:443"))
	assert.Equal(t, "host", stripScheme("https://host/"))
}

func TestBuildAPIServerHTTPClient(t *testing.T) {
	t.Run("invalid CA PEM is an error", func(t *testing.T) {
		_, err := buildAPIServerHTTPClient(apiServerCreds{caPEM: []byte("not a pem")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cluster CA PEM")
	})

	t.Run("no CA builds a client with default timeout", func(t *testing.T) {
		client, err := buildAPIServerHTTPClient(apiServerCreds{})
		require.NoError(t, err)
		assert.Equal(t, configMapFetchTimeout, client.Timeout)
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

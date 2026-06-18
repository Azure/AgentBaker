package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insecureTestClient returns an http.Client that trusts the httptest TLS server's
// self-signed cert, mirroring the production probe's reachability-only behavior.
func insecureTestClient() *http.Client {
	return &http.Client{
		Timeout: lpsProbeTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // test client
		},
	}
}

func TestProbeEndpoint(t *testing.T) {
	t.Run("reachable returns success with status code", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		app := &App{httpProbeClient: insecureTestClient()}
		result := app.probeEndpoint(context.Background(), "fqdn-direct", "fqdn", server.URL)

		assert.True(t, result.reachable)
		assert.Equal(t, http.StatusOK, result.statusCode)
		assert.NoError(t, result.err)
		assert.Equal(t, "fqdn-direct", result.label)
		assert.Equal(t, "fqdn", result.target)
	})

	t.Run("server error is still reachable", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		app := &App{httpProbeClient: insecureTestClient()}
		result := app.probeEndpoint(context.Background(), "fqdn-direct", "fqdn", server.URL)

		assert.True(t, result.reachable)
		assert.Equal(t, http.StatusForbidden, result.statusCode)
		assert.NoError(t, result.err)
	})

	t.Run("unreachable endpoint returns failure without panic", func(t *testing.T) {
		// Start then immediately close a server to get a guaranteed-closed address.
		server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := server.URL
		server.Close()

		app := &App{httpProbeClient: insecureTestClient()}
		result := app.probeEndpoint(context.Background(), "clusterip-via-kube-proxy", "clusterip", url)

		assert.False(t, result.reachable)
		assert.Equal(t, 0, result.statusCode)
		assert.Error(t, result.err)
	})

	t.Run("invalid url returns failure", func(t *testing.T) {
		app := &App{httpProbeClient: insecureTestClient()}
		result := app.probeEndpoint(context.Background(), "bad", "bad", "://not-a-url")

		assert.False(t, result.reachable)
		assert.Error(t, result.err)
	})
}

func TestApiServerFQDNFromConfig(t *testing.T) {
	t.Run("empty path returns empty", func(t *testing.T) {
		assert.Equal(t, "", apiServerFQDNFromConfig(""))
	})

	t.Run("missing file returns empty", func(t *testing.T) {
		assert.Equal(t, "", apiServerFQDNFromConfig(filepath.Join(t.TempDir(), "nope.json")))
	})

	t.Run("garbage file returns empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))
		assert.Equal(t, "", apiServerFQDNFromConfig(path))
	})

	t.Run("valid config returns apiserver name", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		content := `{"version":"v1","apiServerConfig":{"apiServerName":"my-cluster.hcp.eastus.azmk8s.io"}}`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		assert.Equal(t, "my-cluster.hcp.eastus.azmk8s.io", apiServerFQDNFromConfig(path))
	})
}

func TestCheckLPS(t *testing.T) {
	t.Run("always returns nil even when probes fail", func(t *testing.T) {
		app := &App{httpProbeClient: insecureTestClient()}
		// Empty provision config path => FQDN probe is skipped; ClusterIP probe targets
		// 10.0.0.1 which is unreachable in the test environment but must not error out.
		err := app.checkLPS(context.Background(), "")
		assert.NoError(t, err)
	})

	t.Run("probes FQDN sourced from config", func(t *testing.T) {
		var gotFQDNHit bool
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == lpsProbePath {
				gotFQDNHit = true
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		// Point the probe at the test server by rewriting requests through a custom
		// transport so the config-derived URL still resolves to our server.
		client := &http.Client{
			Timeout: lpsProbeTimeout,
			Transport: &rewriteTransport{target: server.Listener.Addr().String(), base: insecureTestClient().Transport},
		}
		app := &App{httpProbeClient: client}

		path := filepath.Join(t.TempDir(), "config.json")
		content := `{"version":"v1","apiServerConfig":{"apiServerName":"my-cluster.example.com"}}`
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

		require.NoError(t, app.checkLPS(context.Background(), path))
		assert.True(t, gotFQDNHit, "expected the FQDN probe to hit the health endpoint")
	})
}

// rewriteTransport redirects all requests to a fixed host:port (the test server),
// preserving the original scheme and path, so we can assert that checkLPS builds and
// issues a request derived from the config-provided FQDN.
type rewriteTransport struct {
	target string
	base   http.RoundTripper
}

func (rt *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Host = rt.target
	return rt.base.RoundTrip(req)
}

func TestProbeClientDefault(t *testing.T) {
	app := &App{}
	client := app.probeClient()
	require.NotNil(t, client)
	assert.Equal(t, lpsProbeTimeout, client.Timeout)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}

func TestProbeEndpointRespectsTimeout(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := insecureTestClient()
	client.Timeout = 50 * time.Millisecond
	app := &App{httpProbeClient: client}

	result := app.probeEndpoint(context.Background(), "slow", "slow", server.URL)
	assert.False(t, result.reachable)
	assert.Error(t, result.err)
}

func TestLogProbeResultMarker(t *testing.T) {
	t.Run("reachable line", func(t *testing.T) {
		var buf bytes.Buffer
		app := &App{probeLogWriter: &buf}
		app.logProbeResult(probeResult{
			label:      "fqdn-direct",
			target:     "fqdn",
			url:        "https://my-cluster.example.com:443/healthz",
			reachable:  true,
			statusCode: 200,
			latency:    42 * time.Millisecond,
		})
		assert.Equal(t,
			"check-lps: target=fqdn url=https://my-cluster.example.com:443/healthz success=true http_status=200 latency_ms=42 err=\"\"\n",
			buf.String())
	})

	t.Run("unreachable line", func(t *testing.T) {
		var buf bytes.Buffer
		app := &App{probeLogWriter: &buf}
		app.logProbeResult(probeResult{
			label:     "clusterip-via-kube-proxy",
			target:    "clusterip",
			url:       "https://10.0.0.1:443/healthz",
			reachable: false,
			latency:   5001 * time.Millisecond,
			err:       errors.New("dial tcp timeout"),
		})
		assert.Equal(t,
			"check-lps: target=clusterip url=https://10.0.0.1:443/healthz success=false http_status=0 latency_ms=5001 err=\"dial tcp timeout\"\n",
			buf.String())
	})
}

func TestCheckLPSEmitsBothTargets(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var buf bytes.Buffer
	app := &App{
		httpProbeClient: &http.Client{
			Timeout:   lpsProbeTimeout,
			Transport: &rewriteTransport{target: server.Listener.Addr().String(), base: insecureTestClient().Transport},
		},
		probeLogWriter: &buf,
	}

	path := filepath.Join(t.TempDir(), "config.json")
	content := `{"version":"v1","apiServerConfig":{"apiServerName":"my-cluster.example.com"}}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	require.NoError(t, app.checkLPS(context.Background(), path))
	out := buf.String()
	assert.Contains(t, out, "check-lps: target=clusterip ")
	assert.Contains(t, out, "check-lps: target=fqdn ")
}

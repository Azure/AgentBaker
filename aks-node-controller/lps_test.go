package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
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
		result := app.probeEndpoint(context.Background(), "fqdn-direct", server.URL)

		assert.True(t, result.reachable)
		assert.Equal(t, http.StatusOK, result.statusCode)
		assert.NoError(t, result.err)
		assert.Equal(t, "fqdn-direct", result.label)
	})

	t.Run("server error is still reachable", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()

		app := &App{httpProbeClient: insecureTestClient()}
		result := app.probeEndpoint(context.Background(), "fqdn-direct", server.URL)

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
		result := app.probeEndpoint(context.Background(), "clusterip-via-kube-proxy", url)

		assert.False(t, result.reachable)
		assert.Equal(t, 0, result.statusCode)
		assert.Error(t, result.err)
	})

	t.Run("invalid url returns failure", func(t *testing.T) {
		app := &App{httpProbeClient: insecureTestClient()}
		result := app.probeEndpoint(context.Background(), "bad", "://not-a-url")

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
		app := &App{
			httpProbeClient:    client,
			fetchAttestedToken: func(context.Context) (string, error) { return "test-token", nil },
		}

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

	result := app.probeEndpoint(context.Background(), "slow", server.URL)
	assert.False(t, result.reachable)
	assert.Error(t, result.err)
}

// genTestCAPEM creates a self-signed CA certificate and returns its PEM encoding.
func genTestCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestProbeLPSFaithful(t *testing.T) {
	t.Run("reachable with auth header and packages path", func(t *testing.T) {
		var gotAuth, gotPath, gotHost string
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			gotHost = r.Host
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"packages":[]}`))
		}))
		defer server.Close()

		client := &http.Client{
			Timeout:   lpsProbeTimeout,
			Transport: &rewriteTransport{target: server.Listener.Addr().String(), base: insecureTestClient().Transport},
		}
		app := &App{
			httpProbeClient:    client,
			fetchAttestedToken: func(context.Context) (string, error) { return "test-token", nil },
		}

		result := app.probeLPSFaithful(context.Background(), "lps-sni-fqdn", "my-cluster.example.com", "")

		assert.True(t, result.reachable)
		assert.Equal(t, http.StatusOK, result.statusCode)
		assert.Equal(t, "lps-sni-fqdn", result.label)
		assert.NoError(t, result.err)
		assert.Equal(t, "test-token", gotAuth, "Authorization header should carry the attested token")
		assert.Equal(t, lpsPackagesPath, gotPath)
		assert.Equal(t, lpsSNIHost+":"+lpsAPIServerPort, gotHost, "Host header should be the LPS SNI host")
		assert.Contains(t, result.url, lpsSNIHost)
	})

	t.Run("token fetch failure is unreachable", func(t *testing.T) {
		app := &App{
			httpProbeClient:    insecureTestClient(),
			fetchAttestedToken: func(context.Context) (string, error) { return "", fmt.Errorf("boom") },
		}
		result := app.probeLPSFaithful(context.Background(), "lps-sni-fqdn", "my-cluster.example.com", "")
		assert.False(t, result.reachable)
		require.Error(t, result.err)
		assert.Contains(t, result.err.Error(), "imds attested token")
	})
}

func TestLpsProbeClient(t *testing.T) {
	t.Run("no config falls back to insecure-skip-verify", func(t *testing.T) {
		app := &App{}
		client, src := app.lpsProbeClient("my-cluster.example.com", "")
		require.NotNil(t, client)
		assert.Equal(t, "insecure-skip-verify", src)

		tr, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		assert.Equal(t, lpsSNIHost, tr.TLSClientConfig.ServerName)
		assert.True(t, tr.TLSClientConfig.InsecureSkipVerify)
		assert.Nil(t, tr.TLSClientConfig.RootCAs)
	})

	t.Run("valid config CA pins RootCAs and verifies", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString(genTestCAPEM(t))
		path := writeConfig(t, fmt.Sprintf(`{"version":"v1","apiServerConfig":{"apiServerName":"my-cluster.example.com"},"kubernetesCaCert":%q}`, encoded))

		app := &App{}
		client, src := app.lpsProbeClient("my-cluster.example.com", path)
		assert.Equal(t, "provision-config", src)

		tr, ok := client.Transport.(*http.Transport)
		require.True(t, ok)
		assert.False(t, tr.TLSClientConfig.InsecureSkipVerify)
		assert.NotNil(t, tr.TLSClientConfig.RootCAs)
		assert.Equal(t, lpsSNIHost, tr.TLSClientConfig.ServerName)
	})

	t.Run("injected client is returned as-is", func(t *testing.T) {
		inj := insecureTestClient()
		app := &App{httpProbeClient: inj}
		client, src := app.lpsProbeClient("my-cluster.example.com", "")
		assert.Equal(t, inj, client)
		assert.Equal(t, "injected", src)
	})
}

func TestCaPoolFromConfig(t *testing.T) {
	t.Run("empty path returns nil", func(t *testing.T) {
		assert.Nil(t, caPoolFromConfig(""))
	})

	t.Run("missing file returns nil", func(t *testing.T) {
		assert.Nil(t, caPoolFromConfig(filepath.Join(t.TempDir(), "nope.json")))
	})

	t.Run("missing CA field returns nil", func(t *testing.T) {
		path := writeConfig(t, `{"version":"v1","apiServerConfig":{"apiServerName":"my-cluster.example.com"}}`)
		assert.Nil(t, caPoolFromConfig(path))
	})

	t.Run("invalid base64 returns nil", func(t *testing.T) {
		path := writeConfig(t, `{"version":"v1","kubernetesCaCert":"not valid base64 !!!"}`)
		assert.Nil(t, caPoolFromConfig(path))
	})

	t.Run("non-cert base64 returns nil", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("not a pem cert"))
		path := writeConfig(t, fmt.Sprintf(`{"version":"v1","kubernetesCaCert":%q}`, encoded))
		assert.Nil(t, caPoolFromConfig(path))
	})

	t.Run("valid CA returns non-nil pool", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString(genTestCAPEM(t))
		path := writeConfig(t, fmt.Sprintf(`{"version":"v1","kubernetesCaCert":%q}`, encoded))
		assert.NotNil(t, caPoolFromConfig(path))
	})
}

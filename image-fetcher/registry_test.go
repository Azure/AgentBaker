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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

//nolint:gocognit
func TestRegistryMirrorTrustAndScopedCredentials(t *testing.T) {
	for _, withCredentials := range []bool{false, true} {
		t.Run(fmt.Sprintf("mirror-credentials=%t", withCredentials), func(t *testing.T) {
			var sourceRequests, mirrorRequests, leakedCredentials atomic.Int32
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				sourceRequests.Add(1)
				http.Error(w, "source must not be used", http.StatusForbidden)
			}))
			defer source.Close()
			sourceHost := strings.TrimPrefix(source.URL, "http://")
			const manifest = `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json"}`
			mirror := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mirrorRequests.Add(1)
				user, password, ok := r.BasicAuth()
				if user == "source-user" || password == "source-password" {
					leakedCredentials.Add(1)
				}
				if !ok || user != "mirror-user" || password != "mirror-password" {
					w.Header().Set("WWW-Authenticate", `Basic realm="mirror"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
				w.Header().Set("Docker-Content-Digest", digest.FromString(manifest).String())
				w.Header().Set("Content-Length", fmt.Sprint(len(manifest)))
				if r.Method != http.MethodHead {
					_, _ = w.Write([]byte(manifest))
				}
			}))
			clientCA, clientCert, clientKey := registryClientCertificate(t)
			mirror.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientCA, MinVersion: tls.VersionTLS12}
			mirror.StartTLS()
			defer mirror.Close()

			root := t.TempDir()
			hostDir := filepath.Join(root, sourceHost)
			if err := os.Mkdir(hostDir, 0700); err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{
				"ca.crt":      pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: mirror.Certificate().Raw}),
				"client.cert": clientCert, "client.key": clientKey,
				"hosts.toml": []byte(fmt.Sprintf("server = %q\nca = 'ca.crt'\nclient = [['client.cert', 'client.key']]\n", mirror.URL)),
			}
			for name, data := range files {
				if err := os.WriteFile(filepath.Join(hostDir, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			config := fmt.Sprintf(`[plugins."io.containerd.cri.v1.images".registry]
config_path = %q
[plugins."io.containerd.cri.v1.images".registry.configs.%q.auth]
username = "source-user"
password = "source-password"
`, root, sourceHost)
			if withCredentials {
				config += fmt.Sprintf(`[plugins."io.containerd.cri.v1.images".registry.configs.%q.auth]
username = "mirror-user"
password = "mirror-password"
`, strings.TrimPrefix(mirror.URL, "https://"))
			}
			resolver, err := registryResolverFromConfig(context.Background(), []byte(config))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, desc, err := resolver.Resolve(ctx, sourceHost+"/test/image:tag")
			if withCredentials {
				if err != nil {
					t.Fatal(err)
				}
				if desc.Digest != digest.FromString(manifest) {
					t.Fatalf("unexpected digest %s", desc.Digest)
				}
			} else if err == nil {
				t.Fatal("mirror without its own credentials must not succeed")
			}
			if sourceRequests.Load() != 0 || leakedCredentials.Load() != 0 {
				t.Fatalf("source requests=%d credential leaks=%d", sourceRequests.Load(), leakedCredentials.Load())
			}
			if mirrorRequests.Load() == 0 {
				t.Fatal("configured TLS mirror was not contacted")
			}
		})
	}
}

func TestInvalidRegistryConfigurationDoesNotContactSource(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	host := strings.TrimPrefix(server.URL, "http://")
	root := t.TempDir()
	dir := filepath.Join(root, host)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf("[plugins.'io.containerd.cri.v1.images'.registry]\nconfig_path = %q", root)
	for _, invalid := range []string{
		"[[[broken", "ca = 123", "client = [[1, 2]]", "client = [['only-one']]",
		"capabilities = ['unknown']", "dial_timeout = 'invalid'", "skip_verify = 'yes'",
		"[header]\nAuthorization = 123", "server = 'https://bad%host'",
	} {
		t.Run(invalid, func(t *testing.T) {
			requests.Store(0)
			if err := os.WriteFile(filepath.Join(dir, "hosts.toml"), []byte(invalid), 0600); err != nil {
				t.Fatal(err)
			}
			resolver, err := registryResolverFromConfig(context.Background(), []byte(config))
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := resolver.Resolve(context.Background(), host+"/image:tag"); err == nil {
				t.Fatal("invalid hosts.toml must fail")
			}
			if requests.Load() != 0 {
				t.Fatal("invalid registry config fell back to the source")
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatal("invalid registry config fell back to the source")
	}
	for _, data := range []string{
		"[[[invalid",
		"[plugins.other]",
		"[plugins.'io.containerd.grpc.v1.cri'.registry.mirrors.'source.test']\nendpoint=['https://mirror.test']",
		"[plugins.'io.containerd.grpc.v1.cri'.registry.configs.'source.test'.tls]\ninsecure_skip_verify=true",
	} {
		if _, err := registryResolverFromConfig(context.Background(), []byte(data)); err == nil {
			t.Fatalf("unsupported registry configuration accepted: %s", data)
		}
	}
}

func TestRegistryDefaultHostFromMultipleConfigRoots(t *testing.T) {
	const manifest = `{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json"}`
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
		w.Header().Set("Docker-Content-Digest", digest.FromString(manifest).String())
		w.Header().Set("Content-Length", fmt.Sprint(len(manifest)))
		_, _ = w.Write([]byte(manifest))
	}))
	defer server.Close()
	root := t.TempDir()
	dir := filepath.Join(root, "_default")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hosts.toml"), []byte(fmt.Sprintf("server = %q", server.URL)), 0600); err != nil {
		t.Fatal(err)
	}
	paths := filepath.Join(t.TempDir(), "absent") + string(os.PathListSeparator) + root
	config := fmt.Sprintf("[plugins.'io.containerd.cri.v1.images'.registry]\nconfig_path = %q", paths)
	resolver, err := registryResolverFromConfig(context.Background(), []byte(config))
	if err != nil {
		t.Fatal(err)
	}
	_, desc, err := resolver.Resolve(context.Background(), "registry.example/test/image:tag")
	if err != nil || desc.Digest != digest.FromString(manifest) {
		t.Fatalf("default host resolution failed: digest=%s err=%v", desc.Digest, err)
	}
	if requests.Load() == 0 {
		t.Fatal("second config root default host was not contacted")
	}
}

func TestRegistryAuthenticationEncodings(t *testing.T) {
	for _, test := range []struct {
		name, user, password string
		auth                 registryAuth
		invalid              bool
	}{
		{name: "empty"},
		{name: "password", user: "user", password: "pass", auth: registryAuth{Username: "user", Password: "pass"}},
		{name: "token", password: "token", auth: registryAuth{IdentityToken: "token"}},
		{name: "encoded", user: "user", password: "pass:word", auth: registryAuth{
			Auth: base64.StdEncoding.EncodeToString([]byte("user:pass:word\x00")),
		}},
		{name: "invalid encoding", auth: registryAuth{Auth: "not-base64"}, invalid: true},
		{name: "missing separator", auth: registryAuth{Auth: base64.StdEncoding.EncodeToString([]byte("user"))}, invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			user, password, err := test.auth.credentials()
			if (err != nil) != test.invalid || user != test.user || password != test.password {
				t.Fatal("unexpected credential decoding result")
			}
		})
	}
}

func registryClientCertificate(t *testing.T) (*x509.CertPool, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "registry-test-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "registry-test-client"},
		NotBefore: ca.NotBefore, NotAfter: ca.NotAfter,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, KeyUsage: x509.KeyUsageDigitalSignature,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, key.Public(), key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}))
	return pool, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

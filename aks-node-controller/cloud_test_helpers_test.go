package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/aks-node-controller/helpers"
	"github.com/stretchr/testify/require"
)

const cloudTestUbuntu = "ID=ubuntu\nNAME=Ubuntu\nVERSION_ID=22.04\nVERSION_CODENAME=jammy\n"

func newTestCloudInitializer(t *testing.T, release string) *cloudInitializer {
	t.Helper()
	c := newCloudInitializer(&App{
		cmdRun: func(cmd *exec.Cmd) error {
			if filepath.Base(cmd.Path) == "gpg" {
				_, _ = io.WriteString(cmd.Stdout, "dearmored-key")
			}
			return nil
		},
		eventLogger: helpers.NewEventLogger(t.TempDir()),
	})
	c.rootDir = t.TempDir()
	c.executable = hotfixBinaryPath
	c.wait = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	require.NoError(t, c.writeFile(defaultOSReleasePath, []byte(release)))
	require.NoError(t, c.writeFile("/etc/ssl/certs/ca-certificates.crt", []byte("system-ca-bundle")))
	t.Cleanup(c.close)
	return c
}

func cloudTestCertificate(t *testing.T) []byte {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ANC cloud initialization test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func serveCloudTestResponses(t *testing.T, c *cloudInitializer, cert []byte, optIn string) <-chan string {
	t.Helper()
	requests := make(chan string, 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.URL.Path + "?" + r.URL.RawQuery
		switch kind := r.URL.Query().Get("type"); {
		case r.URL.Path == "/acms/isOptedInForRootCerts":
			_, _ = io.WriteString(w, optIn)
		case kind == "cacertificates":
			_ = json.NewEncoder(w).Encode(map[string]any{"Certificates": []map[string]string{
				{"Name": "root.cer", "CertBody": strings.ReplaceAll(string(cert), "\n", "\r\n")},
			}})
		case kind == "operationrequestsroot", kind == "operationrequestsintermediate":
			name := "root.crt"
			if kind == "operationrequestsintermediate" {
				name = "intermediate.crt"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"OperationsInfo": []map[string]string{{"ResouceFileName": name}}})
		case kind == "root", kind == "intermediate":
			_, _ = w.Write(cert)
		case strings.HasPrefix(r.URL.Path, "/keys/"):
			_, _ = io.WriteString(w, "armored-public-package-key")
		case strings.HasPrefix(r.URL.Path, "/microsoft/ubuntu/"):
			_, _ = io.WriteString(w, "repository available")
		default:
			t.Errorf("unexpected cloud request: %s", r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	c.wireEndpoint = server.URL
	c.wireClient, c.repoClient = server.Client(), server.Client()
	return requests
}

func serveCloudOperationTestResponses(t *testing.T, c *cloudInitializer, roots, intermediates []string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch kind := r.URL.Query().Get("type"); {
		case r.URL.Path == "/acms/isOptedInForRootCerts":
			_, _ = io.WriteString(w, `{"IsOptedInForRootCerts":true}`)
		case kind == "operationrequestsroot", kind == "operationrequestsintermediate":
			names := roots
			if kind == "operationrequestsintermediate" {
				names = intermediates
			}
			operations := make([]map[string]string, 0, len(names))
			for _, name := range names {
				operations = append(operations, map[string]string{"ResouceFileName": name})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"OperationsInfo": operations})
		case kind == "missing":
			w.WriteHeader(http.StatusServiceUnavailable)
		case kind == "empty":
			_, _ = io.WriteString(w, "\n\n")
		default:
			_, _ = io.WriteString(w, "certificate payload\n\n")
		}
	}))
	t.Cleanup(server.Close)
	c.wireEndpoint = server.URL
}

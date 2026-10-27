package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudLegacyCertificateTransport(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	c.run = func(*exec.Cmd) error {
		t.Fatal("certificate retrieval must not invoke trust-store tools")
		return nil
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if r.URL.Path != "/machine" || query.Get("comp") != "acmspackage" ||
			query.Get("type") != "cacertificates" || query.Get("ext") != "json" {
			t.Errorf("unexpected legacy certificate request: %s", r.URL)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, `{"Certificates":[{"Name":"root.cer","CertBody":"first\r\nsecond\r\n"},{"Name":"../escape.cer","CertBody":"ignored"}]}`)
	}))
	defer server.Close()
	c.wireEndpoint = server.URL
	certificates, err := c.legacyCertificates(t.Context())
	require.NoError(t, err)
	payload := []byte("first\nsecond\r\n")
	require.Equal(t, map[string][]byte{"root.crt": payload}, certificates)
	cached, err := os.ReadFile(c.path("/root/AzureCACertificates/root.crt"))
	require.NoError(t, err)
	require.Equal(t, payload, cached)
	_, err = os.Stat(c.path("/root/escape.crt"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestCloudOperationCertificateTransport(t *testing.T) {
	c := newTestCloudInitializer(t, "ID=azurelinux\n")
	c.run = func(*exec.Cmd) error {
		t.Fatal("certificate retrieval must not invoke trust-store tools")
		return nil
	}
	serveCloudOperationTestResponses(t, c,
		[]string{"missing.crt", "empty.crt", "root+ca.crt", "unfiltered.cer"},
		[]string{"intermediate.crt"})
	certificates, err := c.operationCertificates(t.Context())
	require.NoError(t, err)
	payload := []byte("certificate payload\n")
	require.Equal(t, map[string][]byte{
		"root+ca.crt":      payload,
		"unfiltered.cer":   payload,
		"intermediate.crt": payload,
	}, certificates)
	for name, want := range certificates {
		cached, readErr := os.ReadFile(c.path("/root/AzureCACertificates/" + name))
		require.NoError(t, readErr)
		require.Equal(t, want, cached)
	}
	_, err = os.Stat(c.path("/etc/pki/ca-trust/source/anchors"))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestCloudCertificateTransportEncodesResourceNames(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("type") != "root+ca" || query.Get("ext") != "crt" {
			t.Errorf("incorrectly encoded certificate resource: %s", r.URL)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = io.WriteString(w, "raw certificate payload\n\n")
	}))
	defer server.Close()
	c.wireEndpoint = server.URL
	payload, err := c.operationCertificate(t.Context(), "root+ca.crt")
	require.NoError(t, err)
	require.Equal(t, "raw certificate payload\n", string(payload))
}

package main

import (
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudCertificateStorePlatformInstallation(t *testing.T) {
	for _, tc := range []struct{ id, target, command string }{
		{osReleaseIDUbuntu, "/usr/local/share/ca-certificates/root.crt", "update-ca-certificates"},
		{osReleaseIDMariner, "/etc/pki/ca-trust/source/anchors/root.crt", "update-ca-trust"},
		{osReleaseIDAzureLinux, "/etc/pki/ca-trust/source/anchors/root.crt", "update-ca-trust"},
		{osReleaseIDAzureContainerLinux, "/etc/pki/ca-trust/source/anchors/root.crt", "update-ca-trust"},
		{osReleaseIDFlatcar, "/etc/ssl/certs/root.pem", "update-ca-certificates"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			c := newTestCloudInitializer(t, "ID="+tc.id+"\n")
			platform, err := c.platform()
			require.NoError(t, err)
			var commands []string
			c.run = func(cmd *exec.Cmd) error {
				commands = append(commands, cmd.Args[0])
				return nil
			}
			payload := []byte("unparsed certificate payload\n")
			require.NoError(t, c.installCertificates(t.Context(), platform, map[string][]byte{
				"root.crt": payload,
				"skip.cer": []byte("ignored"),
			}))
			got, err := os.ReadFile(c.path(tc.target))
			require.NoError(t, err)
			require.Equal(t, payload, got)
			require.Equal(t, []string{tc.command}, commands)
			_, err = os.Stat(c.path(filepath.Join(filepath.Dir(tc.target), "skip.cer")))
			require.ErrorIs(t, err, os.ErrNotExist)
			if tc.id == osReleaseIDUbuntu {
				bundle, err := os.ReadFile(c.path("/usr/lib/ssl/cert.pem"))
				require.NoError(t, err)
				require.Equal(t, "system-ca-bundle", string(bundle))
			}
		})
	}
}

func TestCloudCertificateStoreRejectsMissingCRTFiles(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	c.run = func(*exec.Cmd) error {
		t.Fatal("empty certificate sets must not invoke trust-store tools")
		return nil
	}
	err := c.installCertificates(t.Context(), cloudPlatform{ID: osReleaseIDUbuntu}, map[string][]byte{
		"root.cer": []byte("ignored"),
	})
	require.ErrorContains(t, err, "no *.crt files")
}

func TestCloudCertificateStoreClearsOnlyFlatStagingFiles(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	for _, name := range []string{"root.crt", ".hidden.crt", "nested/keep.crt"} {
		require.NoError(t, c.writeFile("/root/AzureCACertificates/"+name, []byte(name)))
	}
	installed := "/usr/local/share/ca-certificates/installed.crt"
	require.NoError(t, c.writeFile(installed, []byte("installed CA")))
	c.clearCertificateCache()
	for _, name := range []string{"root.crt", ".hidden.crt"} {
		_, err := os.Stat(c.path("/root/AzureCACertificates/" + name))
		require.ErrorIs(t, err, os.ErrNotExist)
	}
	for _, name := range []string{"/root/AzureCACertificates/nested/keep.crt", installed} {
		_, err := os.Stat(c.path(name))
		require.NoError(t, err)
	}
}

func TestCloudCertificateStoreLoggingPreservesRawPayload(t *testing.T) {
	t.Setenv("AKS_CLOUD_LOG_CERTIFICATES", "true")
	logs := installLogCapturer(t)
	c := newTestCloudInitializer(t, "ID=azurelinux\n")
	payload := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("not-a-real-private-key")})
	payload = append(payload, cloudTestCertificate(t)...)
	require.NoError(t, c.installCertificates(t.Context(), cloudPlatform{ID: osReleaseIDAzureLinux}, map[string][]byte{
		"root.crt": payload,
	}))
	got, err := os.ReadFile(c.path("/etc/pki/ca-trust/source/anchors/root.crt"))
	require.NoError(t, err)
	require.Equal(t, payload, got)
	found := false
	for _, record := range logs.getRecords() {
		for _, value := range record.Attrs {
			require.NotContains(t, value, "PRIVATE KEY")
			if strings.Contains(value, "BEGIN CERTIFICATE") {
				found = true
			}
		}
	}
	require.True(t, found)
}

package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudFileWrites(t *testing.T) {
	t.Run("create and replace", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "certificates", "root.crt")
		require.NoError(t, writeCloudFile(path, []byte("original")))
		require.NoError(t, writeCloudFile(path, []byte("replacement")))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "replacement", string(data))
	})
	t.Run("reject symlinks outside the destination directory", func(t *testing.T) {
		directory := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.crt")
		require.NoError(t, os.WriteFile(outside, []byte("original"), 0o600))
		path := filepath.Join(directory, "root.crt")
		require.NoError(t, os.Symlink(outside, path))
		require.Error(t, writeCloudFile(path, []byte("replacement")))
		data, err := os.ReadFile(outside)
		require.NoError(t, err)
		require.Equal(t, "original", string(data))
	})
}

func TestCloudPlatform(t *testing.T) {
	for _, tc := range []struct {
		name, release, want string
	}{
		{"Ubuntu", cloudTestUbuntu, osReleaseIDUbuntu},
		{"Mariner", "ID=mariner\nNAME=CBL-Mariner\nVERSION_ID=2.0\n", "mariner"},
		{"AzureLinux", "ID=azurelinux\nNAME=\"Microsoft Azure Linux\"\nVERSION_ID=3.0\n", osReleaseIDAzureLinux},
		{"OSGuard", "ID=azurelinux\nVARIANT_ID=osguard\nVERSION_ID=3.0\n", osReleaseIDAzureLinux},
		{"Flatcar", "ID=flatcar\n", osReleaseIDFlatcar},
		{"ACL", "ID=azurecontainerlinux\n", osReleaseIDAzureContainerLinux},
		{"ACL variant", "ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n", osReleaseIDAzureContainerLinux},
		{"unsupported", "ID=debian\n", ""},
		{"missing ID", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCloudInitializer(t, tc.release)
			platform, err := c.platform()
			if tc.want == "" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, platform.ID)
			}
		})
	}
}

func TestCloudCertificateResponseLimits(t *testing.T) {
	_, err := readCloudResponse(&http.Response{
		Body: io.NopCloser(strings.NewReader(strings.Repeat("x", cloudResponseLimit+1))),
	})
	require.ErrorContains(t, err, "size limit")
}

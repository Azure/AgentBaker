package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudCertificateUbuntuBundleSymlink(t *testing.T) {
	c := newTestCloudInitializer(t, cloudTestUbuntu)
	require.NoError(t, os.MkdirAll(c.path("/usr/lib/ssl"), 0o755))
	require.NoError(t, os.Symlink(c.path("/etc/ssl/certs/ca-certificates.crt"), c.path("/usr/lib/ssl/cert.pem")))
	require.NoError(t, c.copyOpenSSLBundle())
	info, err := os.Lstat(c.path("/usr/lib/ssl/cert.pem"))
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink)
}

package scenario

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

func TestRCV1PCertTrustCommand(t *testing.T) {
	for _, tc := range []struct {
		os        config.OS
		trustDir  string
		bundle    string
		extension string
	}{
		{config.OSUbuntu, "/usr/local/share/ca-certificates", "/etc/ssl/certs/ca-certificates.crt", "crt"},
		{config.OSAzureLinux, "/etc/pki/ca-trust/source/anchors", "/etc/pki/tls/certs/ca-bundle.crt", "crt"},
		{config.OSMariner, "/etc/pki/ca-trust/source/anchors", "/etc/pki/tls/certs/ca-bundle.crt", "crt"},
		{config.OSACL, "/etc/pki/ca-trust/source/anchors", "/etc/pki/tls/certs/ca-bundle.crt", "crt"},
		{config.OSFlatcar, "/etc/ssl/certs", "/etc/ssl/certs/ca-certificates.crt", "pem"},
	} {
		t.Run(string(tc.os), func(t *testing.T) {
			s := &Scenario{Config: Config{VHD: &config.Image{OS: tc.os}}}
			require.Contains(t, rcv1pCertTrustCommand(s),
				"-- /root/AzureCACertificates "+tc.trustDir+" "+tc.bundle+" "+tc.extension)
		})
	}
}

func TestRCV1PCertTrustScript(t *testing.T) {
	bash, err := exec.LookPath("bash")
	require.NoError(t, err)
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("certificate validator execution requires openssl")
	}
	root, rootKey := newRCV1PTestCA(t, nil, nil, "root")
	intermediate, _ := newRCV1PTestCA(t, root, rootKey, "intermediate")
	other, _ := newRCV1PTestCA(t, nil, nil, "other")
	encode := func(cert *x509.Certificate) []byte {
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	}

	for _, tc := range []struct {
		name      string
		extension string
		mutate    func(string, string, string)
		wantError string
	}{
		{name: "root and intermediate trusted", extension: "crt"},
		{name: "flatcar pem names", extension: "pem"},
		{name: "missing installed certificate", extension: "crt", mutate: func(_, trust, _ string) {
			require.NoError(t, os.Remove(filepath.Join(trust, "intermediate.crt")))
		}, wantError: "intermediate.crt"},
		{name: "wrong installed certificate", extension: "crt", mutate: func(_, trust, _ string) {
			require.NoError(t, os.WriteFile(filepath.Join(trust, "root.crt"), encode(other), 0600))
		}, wantError: "fingerprint mismatch"},
		{name: "matching files but stale trust bundle", extension: "crt", mutate: func(_, _, bundle string) {
			require.NoError(t, os.WriteFile(bundle, encode(other), 0600))
		}, wantError: "verification failed"},
		{name: "no downloaded certificates", extension: "crt", mutate: func(download, _, _ string) {
			require.NoError(t, os.Remove(filepath.Join(download, "root.crt")))
			require.NoError(t, os.Remove(filepath.Join(download, "intermediate.crt")))
		}, wantError: "No downloaded RCV1P certificates"},
		{name: "invalid downloaded certificate", extension: "crt", mutate: func(download, _, _ string) {
			require.NoError(t, os.WriteFile(filepath.Join(download, "root.crt"), []byte("invalid"), 0600))
		}, wantError: "certificate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			download, trust, bundle := filepath.Join(dir, "download"), filepath.Join(dir, "trust"), filepath.Join(dir, "bundle.pem")
			require.NoError(t, os.Mkdir(download, 0700))
			require.NoError(t, os.Mkdir(trust, 0700))
			for name, cert := range map[string]*x509.Certificate{"root": root, "intermediate": intermediate} {
				require.NoError(t, os.WriteFile(filepath.Join(download, name+".crt"), encode(cert), 0600))
				require.NoError(t, os.WriteFile(filepath.Join(trust, name+"."+tc.extension), encode(cert), 0600))
			}
			require.NoError(t, os.WriteFile(bundle, encode(root), 0600))
			if tc.mutate != nil {
				tc.mutate(download, trust, bundle)
			}
			output, err := exec.Command(bash, "-c", rcv1pCertTrustScript, "--", download, trust, bundle, tc.extension).CombinedOutput()
			if tc.wantError == "" {
				require.NoError(t, err, "%s", output)
			} else {
				require.Error(t, err, "%s", output)
				require.Contains(t, string(output), tc.wantError)
			}
		})
	}
}

func newRCV1PTestCA(t *testing.T, parent *x509.Certificate, parentKey *ecdsa.PrivateKey, name string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign,
	}
	if parent == nil {
		parent, parentKey = template, key
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, &key.PublicKey, parentKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

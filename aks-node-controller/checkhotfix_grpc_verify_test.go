package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mintCA creates a self-signed CA certificate and returns the parsed cert plus its signing key.
func mintCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return caCert, key
}

// mintLeaf creates a leaf certificate without a SAN/CN, matching the LPS serving certificate.
func mintLeaf(t *testing.T, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	require.NoError(t, err)
	return der
}

// TestVerifyChainAgainstPool asserts that a SAN-less LPS certificate is accepted only when it
// chains to the trusted cluster CA.
func TestVerifyChainAgainstPool(t *testing.T) {
	caCert, caKey := mintCA(t)
	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	leaf := mintLeaf(t, caCert, caKey)

	t.Run("valid SAN-less chain passes", func(t *testing.T) {
		err := verifyChainAgainstPool(pool)([][]byte{leaf}, nil)
		assert.NoError(t, err)
	})

	t.Run("chain to an untrusted CA is rejected", func(t *testing.T) {
		otherCA, _ := mintCA(t)
		untrusted := x509.NewCertPool()
		untrusted.AddCert(otherCA)
		err := verifyChainAgainstPool(untrusted)([][]byte{leaf}, nil)
		require.Error(t, err)
	})

	t.Run("no certificates presented is rejected", func(t *testing.T) {
		err := verifyChainAgainstPool(pool)(nil, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no certificates")
	})
}

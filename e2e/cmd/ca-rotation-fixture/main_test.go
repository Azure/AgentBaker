package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"testing"
)

func TestRegistryTLSAndUniqueImages(t *testing.T) {
	a, err := newAuthority()
	if err != nil {
		t.Fatal(err)
	}
	cert, err := a.serverCertificate(net.ParseIP("127.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	host, _, closeServer, err := startRegistry(net.ParseIP("127.0.0.1"), cert)
	if err != nil {
		t.Fatal(err)
	}
	defer closeServer()
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(a.pem)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}}
	defer client.CloseIdleConnections()
	get := func(path string) ([]byte, string) {
		t.Helper()
		res, err := client.Get("https://" + host + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("get %s: status=%d err=%v", path, res.StatusCode, err)
		}
		return body, res.Header.Get("Docker-Content-Digest")
	}
	first, firstDigest := get("/v2/test/manifests/first")
	_, secondDigest := get("/v2/test/manifests/second")
	if firstDigest == secondDigest || digest(first) != firstDigest {
		t.Fatal("image tags must have distinct, correctly addressed content")
	}
	var manifest struct {
		Config struct {
			Digest string
			Size   int
		}
	}
	if err := json.Unmarshal(first, &manifest); err != nil {
		t.Fatal(err)
	}
	config, _ := get("/v2/test/blobs/" + manifest.Config.Digest)
	if len(config) != manifest.Config.Size || digest(config) != manifest.Config.Digest {
		t.Fatal("manifest config descriptor does not match blob")
	}
	resolved, _ := get("/v2/test/manifests/" + firstDigest)
	if string(resolved) != string(first) {
		t.Fatal("digest resolution changed manifest")
	}
	for _, cfg := range []*tls.Config{
		{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS12},
		{RootCAs: roots, ServerName: "wrong.example.invalid", MinVersion: tls.VersionTLS12},
	} {
		rejected := &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}
		response, err := rejected.Get("https://" + host + "/v2/")
		if err == nil {
			response.Body.Close()
			t.Fatal("expected TLS verification failure")
		}
		rejected.CloseIdleConnections()
	}
}

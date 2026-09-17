package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/internal/carefresh"
)

func TestSyntheticRefreshExpectations(t *testing.T) {
	before := carefresh.Snapshot{
		BundlePath: "/etc/ssl/certs/ca-certificates.crt", BundleSHA256: strings.Repeat("a", 64),
		Kubelet:    carefresh.ServiceIdentity{PID: 1, Started: 1, InvocationID: strings.Repeat("b", 32)},
		Containerd: carefresh.ServiceIdentity{PID: 2, Started: 2, InvocationID: strings.Repeat("c", 32)},
	}
	after := before
	after.BundleSHA256 = strings.Repeat("d", 64)
	after.Containerd = carefresh.ServiceIdentity{PID: 3, Started: 3, InvocationID: strings.Repeat("e", 32)}
	for _, test := range []struct {
		name, output  string
		before, after carefresh.Snapshot
		changed, pass bool
	}{
		{"B installation", "CA_REFRESH_RESULT=restarted", before, after, true, true},
		{"identical B", "CA_REFRESH_RESULT=unchanged", after, after, false, true},
		{"false no-op", "CA_REFRESH_RESULT=unchanged", before, before, true, false},
		{"repeat must not restart", "CA_REFRESH_RESULT=restarted", before, after, false, false},
		{"stdout alone is insufficient", "CA_REFRESH_RESULT=restarted", before, before, true, false},
		{"fresh marker required", "+ echo CA_REFRESH_RESULT=restarted", before, after, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := validateSyntheticRefresh(test.before, test.after, test.output, test.changed)
			if (err == nil) != test.pass {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
}

func TestFixtureRefreshDriver(t *testing.T) {
	// Syntax-check only. Unit tests never execute run(), source the production
	// script, invoke host services, or install certificates in the host trust.
	for _, script := range []string{refresh, trustPaths, carefresh.SnapshotCommand} {
		cmd := exec.Command("bash", "-n")
		cmd.Stdin = strings.NewReader(script)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("invalid fixture shell syntax: %v %s", err, out)
		}
	}
	for _, required := range []string{`__SOURCED__=1 . "$1"`, "refresh_certs() {", "  install_certs_to_trust_store\n}", "refresh_certs_and_containerd fixture"} {
		if !strings.Contains(refresh, required) {
			t.Errorf("fixture driver missing %q", required)
		}
	}
	for _, forbidden := range []string{"RANDOM=", "sleep ", "systemctl ", "update_containerd_ca", "sed -i", "rm -f", "update-ca-certificates", "update-ca-trust"} {
		if strings.Contains(refresh, forbidden) {
			t.Errorf("fixture must not bypass coordinator with %q", forbidden)
		}
	}
}

func TestPolicyEvidence(t *testing.T) {
	dir, err := os.MkdirTemp(".", "policy-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	target := filepath.Join(dir, "hosts.toml")
	link := filepath.Join(dir, "hosts-link.toml")
	if err := os.WriteFile(target, []byte("custom authority"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("hosts.toml", link); err != nil {
		t.Fatal(err)
	}
	file, err := readPolicy(target)
	if err != nil {
		t.Fatal(err)
	}
	symlink, err := readPolicy(link)
	if err != nil || symlink.link != "hosts.toml" || file.contents != symlink.contents || symlink.mode&os.ModeSymlink == 0 {
		t.Fatalf("symlink identity not preserved: %+v %v", symlink, err)
	}
	if err := os.WriteFile(target, []byte("changed authority"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := readPolicy(link)
	if err != nil || changed == symlink {
		t.Fatalf("must detect target content mutation: %v", err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(link, []byte("custom authority"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err = readPolicy(link)
	if err != nil || changed == symlink {
		t.Fatalf("must detect symlink replaced with regular file: %v", err)
	}
}

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

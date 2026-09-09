// ca-rotation-fixture runs only on a disposable AgentBaker scenario node.
// It serves tiny read-only OCI images over TLS and calls the node's real CRI.
// Certificates and keys are generated per execution; no registry or PKI is shared.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const criEndpoint = "unix:///run/containerd/containerd.sock"

type authority struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	pem  []byte
}

func newAuthority() (*authority, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	template := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: "AgentBaker ephemeral E2E CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &authority{cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}, nil
}

func (a *authority) serverCertificate(ip net.IP) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), IPAddresses: []net.IP{ip},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, a.cert, &key.PublicKey, a.key)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, err
}

type registry struct {
	mu       sync.Mutex
	blobs    map[string][]byte
	requests atomic.Int64
}

func digest(data []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(data)) }

func (r *registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.requests.Add(1)
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var body []byte
	switch {
	case req.URL.Path == "/v2/":
		body = []byte("{}")
	case strings.HasPrefix(req.URL.Path, "/v2/test/manifests/"):
		ref := strings.TrimPrefix(req.URL.Path, "/v2/test/manifests/")
		if strings.HasPrefix(ref, "sha256:") {
			body = r.blobs[ref]
		} else {
			// Each tag has a distinct config and manifest digest: a successful
			// pull cannot be explained by an already-cached image.
			config, _ := json.Marshal(map[string]any{
				"architecture": runtime.GOARCH, "os": "linux",
				"config": map[string]any{"Env": []string{"E2E_TAG=" + ref}},
				"rootfs": map[string]any{"type": "layers", "diff_ids": []string{}},
			})
			r.blobs[digest(config)] = config
			body, _ = json.Marshal(map[string]any{
				"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json",
				"config": map[string]any{
					"mediaType": "application/vnd.oci.image.config.v1+json",
					"digest":    digest(config), "size": len(config),
				},
				"layers": []any{},
			})
			r.blobs[digest(body)] = body
		}
		w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
	case strings.HasPrefix(req.URL.Path, "/v2/test/blobs/"):
		body = r.blobs[strings.TrimPrefix(req.URL.Path, "/v2/test/blobs/")]
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if body == nil {
		http.NotFound(w, req)
		return
	}
	w.Header().Set("Docker-Content-Digest", digest(body))
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if req.Method == http.MethodGet {
		_, _ = w.Write(body)
	}
}

func startRegistry(ip net.IP, cert tls.Certificate) (string, *registry, func(), error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(ip.String(), "0"))
	if err != nil {
		return "", nil, nil, err
	}
	r := &registry{blobs: make(map[string][]byte)}
	server := &http.Server{
		Handler: r, ReadHeaderTimeout: 5 * time.Second,
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}},
	}
	go func() {
		if err := server.Serve(tls.NewListener(listener, server.TLSConfig)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("registry server: %v", err)
		}
	}()
	return listener.Addr().String(), r, func() { _ = server.Close() }, nil
}

func command(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s failed: %w: %s", name, err, out)
	}
	return string(out), nil
}

func runtimeIdentity(ctx context.Context) (string, error) {
	out, err := command(ctx, "systemctl", "show", "containerd", "--property=MainPID", "--property=ExecMainStartTimestampMonotonic")
	if err == nil && (strings.Contains(out, "MainPID=0\n") || !strings.Contains(out, "MainPID=")) {
		err = fmt.Errorf("containerd is not running: %s", out)
	}
	return out, err
}

// Use the real trust installation function invoked by ca-refresh. The fixture
// replaces certificate acquisition, not OS trust regeneration or runtime logic.
// Do not contact or change the node's certificate distribution endpoint.
const refresh = `
set -e
__SOURCED__=1 . "$1"
. /etc/os-release
case "$ID" in
  ubuntu) IS_UBUNTU=1 ;;
  mariner) IS_MARINER=1 ;;
  azurelinux) IS_AZURELINUX=1 ;;
  azurecontainerlinux) IS_ACL=1 ;;
  flatcar) IS_FLATCAR=1 ;;
  *) echo "unsupported fixture OS: $ID" >&2; exit 1 ;;
esac
install_certs_to_trust_store
`

const trustPaths = `
. /etc/os-release
case "$ID" in
  ubuntu) printf '/usr/local/share/ca-certificates/%s.crt\nupdate-ca-certificates\n' "$1" ;;
  mariner|azurelinux|azurecontainerlinux) printf '/etc/pki/ca-trust/source/anchors/%s.crt\nupdate-ca-trust\n' "$1" ;;
  flatcar) printf '/etc/ssl/certs/%s.pem\nupdate-ca-certificates\n' "$1" ;;
  *) exit 1 ;;
esac
`

func run(ip net.IP, script, registryScript string) (result error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if os.Geteuid() != 0 || ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return errors.New("run as root on a disposable scenario node with its non-loopback IP (localhost bypasses TLS in containerd)")
	}
	before, err := runtimeIdentity(ctx)
	if err != nil {
		return err
	}
	version, err := command(ctx, "containerd", "--version")
	if err != nil {
		return err
	}
	log.Printf("runtime version: %s", strings.TrimSpace(version))
	log.Printf("runtime before: %s", before)
	defer func() {
		checkCtx, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		after, err := runtimeIdentity(checkCtx)
		if err != nil || after != before {
			result = errors.Join(result, fmt.Errorf("runtime changed: before=%q after=%q error=%v", before, after, err))
		}
		log.Printf("runtime after: %s", after)
	}()
	dir, err := os.MkdirTemp("", "ab-ca-rotation-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(dir)) }()
	a, err := newAuthority()
	if err != nil {
		return err
	}
	b, err := newAuthority()
	if err != nil {
		return err
	}
	certA, err := a.serverCertificate(ip)
	if err != nil {
		return err
	}
	hostA, registryA, closeA, err := startRegistry(ip, certA)
	if err != nil {
		return err
	}
	defer closeA()
	certB, err := b.serverCertificate(ip)
	if err != nil {
		return err
	}
	hostB, registryB, closeB, err := startRegistry(ip, certB)
	if err != nil {
		return err
	}
	defer closeB()
	// Only A has a per-host custom CA. B deliberately uses the unmodified
	// runtime fallback config; installing a test hosts.toml for B would mask the bug.
	hostDir := filepath.Join("/etc/containerd/certs.d", hostA)
	if err := os.MkdirAll(filepath.Dir(hostDir), 0755); err != nil {
		return err
	}
	if err := os.Mkdir(hostDir, 0755); err != nil {
		return err // never overwrite pre-existing configuration
	}
	defer func() { result = errors.Join(result, os.RemoveAll(hostDir)) }()
	caPath := filepath.Join(dir, "a.crt")
	if err := os.WriteFile(caPath, a.pem, 0600); err != nil {
		return err
	}
	// The empty [host] table is required by containerd 1.6/1.7's TOML parser.
	hosts := fmt.Sprintf("server = %q\nca = %q\n[host]\n", "https://"+hostA, caPath)
	if err := os.WriteFile(filepath.Join(hostDir, "hosts.toml"), []byte(hosts), 0644); err != nil {
		return err
	}
	// _default does not overlay explicit host policies. Exercise both the real
	// current AKS generator and migration of its unchanged older layout.
	mirror := filepath.Base(dir) + "-generated.invalid"
	oldMirror := filepath.Base(dir) + "-old.invalid"
	for _, host := range []string{mirror, oldMirror} {
		hostDir := filepath.Join("/etc/containerd/certs.d", host)
		if err := os.Mkdir(hostDir, 0755); err != nil {
			return err
		}
		defer func() { result = errors.Join(result, os.RemoveAll(hostDir)) }()
	}
	const generateMirror = `
set -e
. "$1"
MCR_REPOSITORY_BASE="$2"
BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="$3"
configureContainerdRegistryHost
`
	if _, err := command(ctx, "bash", "-c", generateMirror, "generate-mirror", registryScript, mirror, hostB); err != nil {
		return err
	}
	oldHosts := fmt.Sprintf("[host.%q]\n  capabilities = [\"pull\", \"resolve\"]\n  override_path = true\n", "https://"+hostB+"/v2")
	if err := os.WriteFile(filepath.Join("/etc/containerd/certs.d", oldMirror, "hosts.toml"), []byte(oldHosts), 0644); err != nil {
		return err
	}
	var images []string
	defer func() {
		cleanCtx, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		for _, image := range images {
			_, err := command(cleanCtx, "crictl", "--runtime-endpoint", criEndpoint, "rmi", image)
			result = errors.Join(result, err)
		}
	}()
	pull := func(host, tag string, r *registry, wantX509 bool) error {
		ref := host + "/test:" + filepath.Base(dir) + "-" + tag
		count := r.requests.Load()
		out, err := command(ctx, "crictl", "--runtime-endpoint", criEndpoint, "pull", ref)
		if wantX509 {
			if err == nil || !strings.Contains(out, "x509:") {
				return fmt.Errorf("expected TLS rejection for %s, got: %s / %v", tag, out, err)
			}
			log.Printf("%s: rejected with x509 as expected", tag)
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", tag, err)
		}
		images = append(images, ref)
		if r.requests.Load() <= count {
			return fmt.Errorf("%s: pull succeeded without a registry request", tag)
		}
		log.Printf("%s: CRI pull succeeded; registry requests=%d", tag, r.requests.Load()-count)
		return nil
	}
	if err := pull(hostA, "warm-custom-ca-a", registryA, false); err != nil {
		return err
	}
	if err := pull(hostB, "b-before-refresh", registryB, true); err != nil {
		return err
	}
	name := filepath.Base(dir)
	paths, err := command(ctx, "bash", "-c", trustPaths, "trust-paths", name)
	if err != nil {
		return err
	}
	fields := strings.Fields(paths)
	if len(fields) != 2 {
		return fmt.Errorf("unexpected trust paths: %q", paths)
	}
	staged := filepath.Join("/root/AzureCACertificates", name+".crt")
	if err := os.MkdirAll(filepath.Dir(staged), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(staged, b.pem, 0600); err != nil {
		return err
	}
	defer func() {
		cleanCtx, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		result = errors.Join(result, os.Remove(staged))
		if err := os.Remove(fields[0]); err != nil && !os.IsNotExist(err) {
			result = errors.Join(result, err)
		}
		_, err := command(cleanCtx, fields[1])
		result = errors.Join(result, err)
		if fields[1] == "update-ca-certificates" && strings.HasPrefix(fields[0], "/usr/local/share/") {
			_, err := command(cleanCtx, "bash", "-c", `if [ ! /etc/ssl/certs/ca-certificates.crt -ef /usr/lib/ssl/cert.pem ]; then cp /etc/ssl/certs/ca-certificates.crt /usr/lib/ssl/cert.pem; fi`)
			result = errors.Join(result, err)
		}
	}()
	if out, err := command(ctx, "bash", "-c", refresh, "refresh", script); err != nil {
		return fmt.Errorf("OS trust refresh: %w: %s", err, out)
	}
	// A new process proves the OS trust update really succeeded independently
	// of containerd's long-lived Go root pool.
	if _, err := command(ctx, "curl", "--noproxy", "*", "--fail", "--silent", "--show-error", "https://"+hostB+"/v2/"); err != nil {
		return fmt.Errorf("fresh OS trust client: %w", err)
	}
	log.Print("fresh OS trust client: B accepted")
	if err := pull(hostB, "b-after-refresh", registryB, false); err != nil {
		return err
	}
	if err := pull(mirror, "generated-mirror-after-refresh", registryB, false); err != nil {
		return err
	}
	if err := pull(oldMirror, "older-mirror-after-refresh", registryB, false); err != nil {
		return err
	}
	if err := pull(hostA, "custom-ca-a-preserved", registryA, false); err != nil {
		return err
	}
	// Trusted CA, wrong SAN: adding trust must not disable hostname verification.
	wrongCert, err := b.serverCertificate(net.ParseIP("192.0.2.1"))
	if err != nil {
		return err
	}
	wrongHost, wrongRegistry, closeWrong, err := startRegistry(ip, wrongCert)
	if err != nil {
		return err
	}
	defer closeWrong()
	if err := pull(wrongHost, "wrong-san", wrongRegistry, true); err != nil {
		return err
	}
	// A is deliberately NOT in OS trust. An unconfigured A endpoint must
	// remain untrusted despite the separate custom-CA namespace for hostA.
	untrustedHost, untrustedRegistry, closeUntrusted, err := startRegistry(ip, certA)
	if err != nil {
		return err
	}
	defer closeUntrusted()
	return pull(untrustedHost, "untrusted-ca", untrustedRegistry, true)
}

func main() {
	ip := flag.String("node-ip", "", "private IP of the disposable scenario node")
	script := flag.String("refresh-script", "", "branch init-aks-cloud.sh containing the real trust installer")
	registryScript := flag.String("registry-script", "", "branch cse_config.sh containing the real registry generator")
	flag.Parse()
	if *script == "" || *registryScript == "" {
		log.Fatal("--refresh-script and --registry-script are required")
	}
	if err := run(net.ParseIP(*ip), *script, *registryScript); err != nil {
		log.Fatal(err)
	}
	log.Print("PASS: refreshed trust used by CRI without restarting containerd")
}

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Run with IMAGE_FETCHER_RUNTIME_TEST=1 inside an isolated user/mount namespace.
//
//nolint:cyclop,gocognit,gocyclo
func TestPullImageRuntimeIntegration(t *testing.T) {
	if os.Getenv("IMAGE_FETCHER_RUNTIME_TEST") != "1" {
		t.Skip("requires an isolated containerd and mount-capable user namespace")
	}
	if os.Geteuid() != 0 {
		t.Fatal("run the integration test in an isolated user/mount namespace")
	}
	blobs, manifest, layer := registryTestImage(t)
	var sourceRequests, mirrorRequests atomic.Int32
	var denyRequests atomic.Bool
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sourceRequests.Add(1)
		w.WriteHeader(http.StatusForbidden)
	}))
	defer source.Close()
	mirror := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mirrorRequests.Add(1)
		user, password, ok := r.BasicAuth()
		if !ok || user != "runtime-user" || password != "runtime-password" || denyRequests.Load() {
			w.Header().Set("WWW-Authenticate", `Basic realm="runtime-mirror"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var data []byte
		switch {
		case strings.HasPrefix(r.URL.Path, "/v2/test/image/manifests/"):
			data = manifest
			w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
		case strings.HasPrefix(r.URL.Path, "/v2/test/image/blobs/"):
			data = blobs[strings.TrimPrefix(r.URL.Path, "/v2/test/image/blobs/")]
			w.Header().Set("Content-Type", "application/octet-stream")
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if data == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Docker-Content-Digest", digest.FromBytes(data).String())
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
	}))
	defer mirror.Close()
	root := t.TempDir()
	sourceHost := strings.TrimPrefix(source.URL, "http://")
	mirrorHost := strings.TrimPrefix(mirror.URL, "https://")
	hostRoot := filepath.Join(root, "hosts")
	hostDir := filepath.Join(hostRoot, sourceHost)
	if err := os.MkdirAll(hostDir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"hosts.toml": []byte(fmt.Sprintf("server = %q\nca = 'ca.crt'\n", mirror.URL)),
		"ca.crt": pem.EncodeToMemory(&pem.Block{
			Type: "CERTIFICATE", Bytes: mirror.Certificate().Raw,
		}),
	} {
		if err := os.WriteFile(filepath.Join(hostDir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	config := fmt.Sprintf(`version = 2
root = %q
state = %q
[grpc]
address = %q
[plugins."io.containerd.grpc.v1.cri".containerd]
snapshotter = "native"
[plugins."io.containerd.grpc.v1.cri".registry]
config_path = %q
[plugins."io.containerd.grpc.v1.cri".registry.configs.%q.auth]
username = "runtime-user"
password = "runtime-password"
`, filepath.Join(root, "root"), filepath.Join(root, "state"), filepath.Join(root, "containerd.sock"), hostRoot, mirrorHost)
	client := startRegistryTestRuntime(t, root, config)
	ctx, cancel := context.WithTimeout(namespaces.WithNamespace(context.Background(), "image-cache-review"), 45*time.Second)
	defer cancel()
	ref := sourceHost + "/test/image:tag"
	assertResult := func(want string) {
		t.Helper()
		result, err := pullAndUnpackImage(ctx, client, ref)
		if err != nil {
			t.Fatal(err)
		}
		if result.Outcome != want || result.Snapshotter != "native" || result.Digest != digest.FromBytes(manifest).String() {
			t.Fatalf("unexpected result: %+v", result)
		}
		image, err := client.GetImage(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}
		unpacked, err := image.IsUnpacked(ctx, "native")
		if err != nil || !unpacked {
			t.Fatalf("independent snapshot check failed: unpacked=%t err=%v", unpacked, err)
		}
		reader, err := client.ContentStore().ReaderAt(ctx, ocispec.Descriptor{Digest: layer})
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(content.NewReader(reader))
		_ = reader.Close()
		if err != nil || digest.FromBytes(data) != layer {
			t.Fatalf("independent compressed-layer check failed: %v", err)
		}
	}
	assertResult("pulled")
	requestsAfterCold := mirrorRequests.Load()
	if requestsAfterCold == 0 {
		t.Fatal("cold pull did not reach the configured mirror")
	}
	denyRequests.Store(true)
	assertResult("cache-hit")
	image, err := client.GetImage(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	diffIDs, err := image.RootFS(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SnapshotService("native").Remove(ctx, identity.ChainID(diffIDs).String()); err != nil {
		t.Fatal(err)
	}
	assertResult("unpacked")
	if mirrorRequests.Load() != requestsAfterCold {
		t.Fatal("warm or local-only unpack contacted the registry")
	}
	denyRequests.Store(false)
	if err := client.ContentStore().Delete(ctx, layer); err != nil {
		t.Fatal(err)
	}
	assertResult("pulled")
	if mirrorRequests.Load() <= requestsAfterCold {
		t.Fatal("missing compressed content was not repaired from the mirror")
	}
	denyRequests.Store(true)
	if _, err := pullAndUnpackImage(ctx, client, sourceHost+"/test/image:denied"); err == nil {
		t.Fatal("denied registry credentials must not report readiness")
	}
	if sourceRequests.Load() != 0 {
		t.Fatal("mirror-only runtime fixture contacted the source registry")
	}
}

func startRegistryTestRuntime(t *testing.T, root, config string) *containerd.Client {
	t.Helper()
	binary, err := exec.LookPath("containerd")
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.toml")
	if err = os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "containerd.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "--config", configPath)
	cmd.Stdout, cmd.Stderr = log, log
	if err = cmd.Start(); err != nil {
		_ = log.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = log.Close()
		if t.Failed() {
			data, _ := os.ReadFile(logPath)
			t.Logf("isolated containerd log:\n%s", data)
		}
	})
	binDir := filepath.Join(root, "bin")
	if err = os.Mkdir(binDir, 0700); err != nil {
		t.Fatal(err)
	}
	shim := fmt.Sprintf("#!/bin/sh\nexec %q --config %q \"$@\"\n", binary, configPath)
	if err = os.WriteFile(filepath.Join(binDir, "containerd"), []byte(shim), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	client, err := containerd.New(filepath.Join(root, "containerd.sock"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		if _, err := client.Version(ctx); err == nil {
			return client
		}
		select {
		case <-ctx.Done():
			t.Fatal("isolated containerd failed readiness")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func registryTestImage(t *testing.T) (map[string][]byte, []byte, digest.Digest) {
	t.Helper()
	var raw, compressed bytes.Buffer
	writer := tar.NewWriter(&raw)
	if err := writer.WriteHeader(&tar.Header{Name: "hello.txt", Mode: 0644, Size: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := gzipWriter.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	layer := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip, Digest: digest.FromBytes(compressed.Bytes()), Size: int64(compressed.Len()),
	}
	config, err := json.Marshal(ocispec.Image{
		Platform: ocispec.Platform{OS: "linux", Architecture: runtime.GOARCH},
		RootFS:   ocispec.RootFS{Type: "layers", DiffIDs: []digest.Digest{digest.FromBytes(raw.Bytes())}},
	})
	if err != nil {
		t.Fatal(err)
	}
	configDesc := ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig, Digest: digest.FromBytes(config), Size: int64(len(config))}
	manifest, err := json.Marshal(ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2}, MediaType: ocispec.MediaTypeImageManifest,
		Config: configDesc, Layers: []ocispec.Descriptor{layer},
	})
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{configDesc.Digest.String(): config, layer.Digest.String(): compressed.Bytes()}, manifest, layer.Digest
}

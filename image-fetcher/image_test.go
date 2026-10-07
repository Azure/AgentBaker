package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSnapshotter(t *testing.T) {
	t.Setenv("CONTAINERD_SNAPSHOTTER", "")
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[plugins."io.containerd.grpc.v1.cri".containerd]
  snapshotter = "overlaybd"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := detectSnapshotter(path); got != "overlaybd" {
		t.Fatalf("detectSnapshotter() = %q, want overlaybd", got)
	}
}

func TestDetectSnapshotterV2Config(t *testing.T) {
	t.Setenv("CONTAINERD_SNAPSHOTTER", "")
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`
[plugins.'io.containerd.cri.v1.images']
  snapshotter = 'erofs'
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := detectSnapshotter(path); got != "erofs" {
		t.Fatalf("detectSnapshotter() = %q, want erofs", got)
	}
}

func TestDetectSnapshotterOverrideAndFallback(t *testing.T) {
	t.Setenv("CONTAINERD_SNAPSHOTTER", "native")
	if got := detectSnapshotter(filepath.Join(t.TempDir(), "missing")); got != "native" {
		t.Fatalf("detectSnapshotter() = %q, want native", got)
	}

	t.Setenv("CONTAINERD_SNAPSHOTTER", "")
	if got := detectSnapshotter(filepath.Join(t.TempDir(), "missing")); got != defaultSnapshotter {
		t.Fatalf("detectSnapshotter() = %q, want %q", got, defaultSnapshotter)
	}
}

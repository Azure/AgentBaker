package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/plugins/content/local"
	"github.com/containerd/platforms"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestStrictContentRejectsMissingLayerDespiteReportedSize(t *testing.T) {
	ctx := context.Background()
	store, err := local.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	platform := ocispec.Platform{OS: "linux", Architecture: "amd64"}
	matcher := platforms.OnlyStrict(platform)
	configData, err := json.Marshal(ocispec.Image{Platform: platform})
	if err != nil {
		t.Fatal(err)
	}
	config := ocispec.Descriptor{MediaType: ocispec.MediaTypeImageConfig, Digest: digest.FromBytes(configData), Size: int64(len(configData))}
	layerData := []byte("test compressed layer content")
	layer := ocispec.Descriptor{MediaType: ocispec.MediaTypeImageLayerGzip, Digest: digest.FromBytes(layerData), Size: int64(len(layerData))}
	manifestData, err := json.Marshal(ocispec.Manifest{
		Versioned: specs.Versioned{SchemaVersion: 2},
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    config, Layers: []ocispec.Descriptor{layer},
	})
	if err != nil {
		t.Fatal(err)
	}
	target := ocispec.Descriptor{MediaType: ocispec.MediaTypeImageManifest, Digest: digest.FromBytes(manifestData), Size: int64(len(manifestData))}
	for index, blob := range []struct {
		descriptor ocispec.Descriptor
		data       []byte
	}{{config, configData}, {target, manifestData}} {
		if err := content.WriteBlob(ctx, store, string(rune('a'+index)), bytes.NewReader(blob.data), blob.descriptor); err != nil {
			t.Fatal(err)
		}
	}
	image := images.Image{Target: target}
	if _, err := image.Size(ctx, store, matcher); err != nil {
		t.Fatalf("manifest-reported size unexpectedly detected missing layer: %v", err)
	}
	if err := checkImageContent(ctx, store, target, matcher); err == nil {
		t.Fatal("strict readiness accepted a missing layer")
	}
	if err := content.WriteBlob(ctx, store, "layer", bytes.NewReader(layerData), layer); err != nil {
		t.Fatal(err)
	}
	if err := checkImageContent(ctx, store, target, matcher); err != nil {
		t.Fatalf("complete native-platform content was rejected: %v", err)
	}
}

func TestSnapshotterFromEffectiveConfig(t *testing.T) {
	for _, test := range []struct {
		name   string
		config string
		want   string
	}{
		{
			name: "legacy",
			config: `[plugins."io.containerd.grpc.v1.cri".containerd]
snapshotter = "overlayfs"
`,
			want: "overlayfs",
		},
		{
			name: "legacy runtime override",
			config: `[plugins."io.containerd.grpc.v1.cri".containerd]
snapshotter = "overlayfs"
default_runtime_name = "runc"
[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
snapshotter = "native"
`,
			want: "native",
		},
		{
			name: "split CRI",
			config: `[plugins."io.containerd.cri.v1.images"]
snapshotter = "overlayfs"
`,
			want: "overlayfs",
		},
		{
			name: "split CRI runtime override",
			config: `[plugins."io.containerd.cri.v1.images"]
snapshotter = "overlayfs"
[plugins."io.containerd.cri.v1.images".runtime_platforms.runc]
snapshotter = "native"
[plugins."io.containerd.cri.v1.runtime".containerd]
default_runtime_name = "runc"
`,
			want: "native",
		},
		{name: "missing snapshotter", config: `[plugins."io.containerd.cri.v1.images"]`},
		{name: "invalid TOML", config: `[[[invalid`},
		{name: "unrelated snapshotter", config: `[plugins.other]
snapshotter = "overlayfs"`},
		{name: "do not fall back from invalid split CRI", config: `[plugins."io.containerd.cri.v1.images"]
[plugins."io.containerd.grpc.v1.cri".containerd]
snapshotter = "overlayfs"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshotter, err := snapshotterFromConfig([]byte(test.config))
			if test.want == "" {
				if err == nil {
					t.Fatal("expected invalid CRI configuration to fail")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if snapshotter != test.want {
					t.Fatalf("snapshotter = %q, want %q", snapshotter, test.want)
				}
			}
		})
	}
}

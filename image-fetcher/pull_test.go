package main

import (
	"testing"
)

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

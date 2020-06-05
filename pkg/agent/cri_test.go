package agent

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
)

func TestGetDockerConfig(t *testing.T) {
	tests := []struct {
		name      string
		options   map[string]string
		overrides []func(*DockerConfig) error
		fail      bool
		want      string
	}{
		{
			name: "docker default config",
			want: defaultDockerConfigString,
			fail: false,
		},
		{
			name: "docker reroot config",
			want: dockerRerootConfigString,
			fail: false,
			options: map[string]string{
				"dataDir": "/mnt/docker",
			},
		},
		{
			name: "docker nvidia config",
			want: dockerNvidiaConfigString,
			fail: false,
			overrides: []func(*DockerConfig) error{
				DockerNvidiaOverride,
			},
		},
		{
			name: "docker force error",
			want: "",
			fail: true,
			overrides: []func(*DockerConfig) error{
				func(_ *DockerConfig) error {
					return errors.New("foo")
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetDockerConfig(test.options, test.overrides)
			if err != nil && !test.fail {
				t.Fatalf("failed to get docker config: %v", err)
			}
			if test.fail {
				if err == nil {
					t.Fatalf("got docker config successfully while expecting failure")
				}
			} else {
				if err != nil {
					t.Fatalf("failed to get docker config: %v", err)
				}
				diff := cmp.Diff(test.want, got)
				if diff != "" {
					t.Fatalf(diff)
				}
			}
		})
	}
}

func TestGetContainerdConfig(t *testing.T) {
	tests := []struct {
		name      string
		options   map[string]string
		overrides []func(*ContainerdConfig) error
		fail      bool
		want      string
	}{
		{
			name: "container default config",
			want: defaultContainerdConfigString,
			fail: false,
		},

		{
			name: "container reroot config",
			want: containerdRerootConfigString,
			fail: false,
			options: map[string]string{
				"dataDir": "/mnt/containerd",
			},
		},
		{
			name: "container kubenet config",
			want: containerdKubenetConfigString,
			fail: false,
			overrides: []func(*ContainerdConfig) error{
				ContainerdKubenetOverride,
			},
		},
		{
			name: "container sandbox image config",
			want: containerdImageConfigString,
			fail: false,
			overrides: []func(*ContainerdConfig) error{
				ContainerdSandboxImageOverrider("foo/k8s/core/pause:1.2.0"),
			},
		},
		{
			name: "container force error",
			want: "",
			fail: true,
			overrides: []func(*ContainerdConfig) error{
				func(_ *ContainerdConfig) error {
					return errors.New("foo")
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetContainerdConfig(test.options, test.overrides)
			if err != nil && !test.fail {
				t.Fatalf("failed to get docker config: %v", err)
			}
			if test.fail {
				if err == nil {
					t.Fatalf("got docker config successfully while expecting failure")
				}
			} else {
				if err != nil {
					t.Fatalf("failed to get docker config: %v", err)
				}
				diff := cmp.Diff(test.want, got)
				if diff != "" {
					t.Fatalf(diff)
				}
			}
		})
	}
}

func TestIndentString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		count int
		want  string
	}{
		{
			name:  "should leave empty string alone",
			input: "",
			count: 4,
			want:  "",
		},
		{
			name:  "should indent single line string 4 spaces",
			input: "foo",
			count: 4,
			want:  "    foo\n",
		},
		{
			name:  "should indent multi-line string 4 spaces",
			input: "foo\nbar",
			count: 4,
			want:  "    foo\n    bar\n",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := IndentString(test.input, test.count)
			diff := cmp.Diff(test.want, got)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

var defaultContainerdConfigString = `oom_score = 0
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".cni]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdRerootConfigString = `oom_score = 0
root = "/mnt/containerd"
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".cni]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdKubenetConfigString = `oom_score = 0
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".cni]
      conf_template = "/etc/containerd/kubenet_template.conf"
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdImageConfigString = `oom_score = 0
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "foo/k8s/core/pause:1.2.0"
    [plugins."io.containerd.grpc.v1.cri".cni]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdImageRerootConfigString = `oom_score = 0
root = "/mnt/containerd"
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "foo/k8s/core/pause:1.2.0"
    [plugins."io.containerd.grpc.v1.cri".cni]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdImageKubenetConfigString = `oom_score = 0
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "foo/k8s/core/pause:1.2.0"
    [plugins."io.containerd.grpc.v1.cri".cni]
      conf_template = "/etc/containerd/kubenet_template.conf"
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var containerdAllConfigString = `oom_score = 0
root = "/mnt/containerd"
version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "foo/k8s/core/pause:1.2.0"
    [plugins."io.containerd.grpc.v1.cri".cni]
      conf_template = "/etc/containerd/kubenet_template.conf"
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          runtime_type = "io.containerd.runc.v2"
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.untrusted]
          runtime_type = "io.containerd.runc.v2"
`

var defaultDockerConfigString = `{
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "5"
    }
}`

var dockerRerootConfigString = `{
    "data-root": "/mnt/docker",
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "5"
    }
}`

var dockerNvidiaConfigString = `{
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "5"
    },
    "default-runtime": "nvidia",
    "runtimes": {
        "nvidia": {
            "path": "/usr/bin/nvidia-container-runtime",
            "runtimeArgs": []
        }
    }
}`

var dockerAllConfigString = `{
    "data-root": "/mnt/docker",
    "live-restore": true,
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "5"
    },
    "default-runtime": "nvidia",
    "runtimes": {
        "nvidia": {
            "path": "/usr/bin/nvidia-container-runtime",
            "runtimeArgs": []
        }
    }
}`

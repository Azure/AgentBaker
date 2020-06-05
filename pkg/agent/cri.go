package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"github.com/BurntSushi/toml"
)

type ContainerdConfig struct {
	OomScore int     `toml:"oom_score,omitempty"`
	Root     string  `toml:"root,omitempty"`
	Version  int     `toml:"version,omitempty"`
	Plugins  Plugins `toml:"plugins,omitempty"`
}

type ContainerdCNIPlugin struct {
	ConfTemplate string `toml:"conf_template,omitempty"`
}

type ContainerdRuntime struct {
	RuntimeType string `toml:"runtime_type,omitempty"`
}

type ContainerdPlugin struct {
	DefaultRuntimeName string                       `toml:"default_runtime_name,omitempty"`
	Runtimes           map[string]ContainerdRuntime `toml:"runtimes,omitempty"`
}

type IoContainerdGrpcV1Cri struct {
	SandboxImage string              `toml:"sandbox_image,omitempty"`
	CNI          ContainerdCNIPlugin `toml:"cni,omitempty"`
	Containerd   ContainerdPlugin    `toml:"containerd,omitempty"`
}

type Plugins struct {
	IoContainerdGrpcV1Cri IoContainerdGrpcV1Cri `toml:"io.containerd.grpc.v1.cri,omitempty"`
}

type DockerConfig struct {
	ExecOpts             []string                       `json:"exec-opts,omitempty"`
	DataRoot             string                         `json:"data-root,omitempty"`
	LiveRestore          bool                           `json:"live-restore,omitempty"`
	LogDriver            string                         `json:"log-driver,omitempty"`
	LogOpts              LogOpts                        `json:"log-opts,omitempty"`
	DefaultRuntime       string                         `json:"default-runtime,omitempty"`
	DockerDaemonRuntimes map[string]DockerDaemonRuntime `json:"runtimes,omitempty"`
}

type LogOpts struct {
	MaxSize string `json:"max-size,omitempty"`
	MaxFile string `json:"max-file,omitempty"`
}

type DockerDaemonRuntime struct {
	Path        string   `json:"path,omitempty"`
	RuntimeArgs []string `json:"runtimeArgs"`
}

var (
	// DefaultDockerConfig describes the default configuration of the docker daemon.
	DefaultDockerConfig = DockerConfig{
		LiveRestore: true,
		LogDriver:   "json-file",
		LogOpts: LogOpts{
			MaxSize: "50m",
			MaxFile: "5",
		},
	}

	// DefaultContainerdConfig describes the default configuration of the containerd daemon.
	DefaultContainerdConfig = ContainerdConfig{
		Version:  2,
		OomScore: 0,
		Plugins: Plugins{
			IoContainerdGrpcV1Cri: IoContainerdGrpcV1Cri{
				CNI: ContainerdCNIPlugin{},
				Containerd: ContainerdPlugin{
					DefaultRuntimeName: "runc",
					Runtimes: map[string]ContainerdRuntime{
						"runc": {
							RuntimeType: "io.containerd.runc.v2",
						},
						// note: runc really should not be used for untrusted workloads... should we remove this? This is here because it was here before
						"untrusted": {
							RuntimeType: "io.containerd.runc.v2",
						},
					},
				},
			},
		},
	}
)

// GetDefaultDockerConfig returns the default docker config for processing.
func GetDefaultDockerConfig() DockerConfig {
	return DefaultDockerConfig
}

// GetDefaultContainerdConfig returns the default containerd config for processing.
func GetDefaultContainerdConfig() ContainerdConfig {
	return DefaultContainerdConfig
}

// Known container runtime configuration keys
const (
	ContainerDataDirKey = "dataDir"
)

// GetDockerConfig transforms the default docker config with overrides. Overrides may be nil.
func GetDockerConfig(opts map[string]string, overrides []func(*DockerConfig) error) (string, error) {
	config := GetDefaultDockerConfig()

	for i := range overrides {
		if err := overrides[i](&config); err != nil {
			return "", err
		}
	}

	dataDir, ok := opts[ContainerDataDirKey]
	if ok {
		config.DataRoot = dataDir
	}

	b, err := json.MarshalIndent(config, "", "    ")
	return string(b), err
}

// GetContainerdConfig transforms the default containerd config with overrides. Overrides may be nil.
func GetContainerdConfig(opts map[string]string, overrides []func(*ContainerdConfig) error) (string, error) {
	config := GetDefaultContainerdConfig()

	for i := range overrides {
		if err := overrides[i](&config); err != nil {
			return "", err
		}
	}

	dataDir, ok := opts[ContainerDataDirKey]
	if ok {
		config.Root = dataDir
	}

	buf := new(bytes.Buffer)
	err := toml.NewEncoder(buf).Encode(config)
	return buf.String(), err
}

// ContainerdKubenetOverride transforms a containerd config to set details required when using kubenet.
func ContainerdKubenetOverride(config *ContainerdConfig) error {
	config.Plugins.IoContainerdGrpcV1Cri.CNI.ConfTemplate = "/etc/containerd/kubenet_template.conf"
	return nil
}

// ContainerdSandboxImageOverrider produces a function to transform containerd config by setting the SandboxImage.
func ContainerdSandboxImageOverrider(image string) func(*ContainerdConfig) error {
	return func(config *ContainerdConfig) error {
		config.Plugins.IoContainerdGrpcV1Cri.SandboxImage = image
		return nil
	}
}

// DockerNvidiaOverride transforms a docker config to supply nvidia runtime configuration.
func DockerNvidiaOverride(config *DockerConfig) error {
	if config.DockerDaemonRuntimes == nil {
		config.DockerDaemonRuntimes = make(map[string]DockerDaemonRuntime)
	}
	config.DefaultRuntime = "nvidia"
	config.DockerDaemonRuntimes["nvidia"] = DockerDaemonRuntime{
		Path:        "/usr/bin/nvidia-container-runtime",
		RuntimeArgs: []string{},
	}
	return nil
}

// IndentString pads each line of an original string with N spaces and returns the new value.
func IndentString(original string, spaces int) string {
	out := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(strings.NewReader(original))
	for scanner.Scan() {
		for i := 0; i < spaces; i++ {
			out.WriteString(" ")
		}
		out.WriteString(scanner.Text())
		out.WriteString("\n")
	}
	return out.String()
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	yaml "sigs.k8s.io/yaml/goyaml.v3" // TODO: should we use JSON instead of YAML to avoid 3rd party dependencies?

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// Some options are intentionally non-configurable to avoid customization by users
// it will help us to avoid introducing any breaking changes in the future.
const (
	ConfigVersion                  = "v0"
	DefaultAksAadAppID             = "6dae42f8-4368-4678-94ff-3960e28e3630"
	LogFile                        = "/var/log/azure/node-bootstrapper.log"
	ReadOnlyUser       os.FileMode = 0600
	ReadOnlyWorld      os.FileMode = 0644
)

type Config struct {
	Version string `json:"version"`
}

// SensitiveString is a custom type for sensitive information, like passwords or tokens.
// It reduces the risk of leaking sensitive information in logs.
type SensitiveString string

// String implements the fmt.Stringer interface.
func (s SensitiveString) String() string {
	return "[REDACTED]"
}

func (s SensitiveString) LogValue() slog.Value {
	return slog.StringValue(s.String())
}

func (s SensitiveString) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

//nolint:unparam // this is an interface implementation
func (s SensitiveString) MarshalYAML() (interface{}, error) {
	return s.String(), nil
}

func (s SensitiveString) UnsafeValue() string {
	return string(s)
}

func main() {
	logFile, err := os.OpenFile(LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		//nolint:forbidigo // there is no other way to communicate the error
		fmt.Printf("failed to open log file: %s\n", err)
		os.Exit(1)
	}
	//nolint:gocritic // TODO: ensure log file is closed before exiting from errors
	defer logFile.Close()

	logger := slog.New(slog.NewJSONHandler(logFile, nil))
	slog.SetDefault(logger)

	slog.Info("node-bootstrapper started")
	ctx := context.Background()
	if err := Run(ctx); err != nil {
		slog.Error("node-bootstrapper finished with error", "error", err.Error())
		var exitErr *exec.ExitError
		_ = logFile.Close()
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	slog.Info("node-bootstrapper finished")
}

func Run(ctx context.Context) error {
	const minNumberArgs = 2
	if len(os.Args) < minNumberArgs {
		return errors.New("missing command argument")
	}
	switch os.Args[1] {
	case "provision":
		return Provision(ctx)
	default:
		return fmt.Errorf("unknown command: %s", os.Args[1])
	}
}

// usage example:
// node-bootstrapper provision --provision-config=config.json .
func Provision(ctx context.Context) error {
	fs := flag.NewFlagSet("provision", flag.ContinueOnError)
	provisionConfig := fs.String("provision-config", "", "path to the provision config file")
	err := fs.Parse(os.Args[2:])
	if err != nil {
		return fmt.Errorf("parse args: %w", err)
	}
	if provisionConfig == nil || *provisionConfig == "" {
		return errors.New("--provision-config is required")
	}

	config, err := loadConfig(*provisionConfig)
	if err != nil {
		return err
	}

	if err := writeCustomData(config); err != nil {
		return fmt.Errorf("write custom data: %w", err)
	}

	if err := provisionStart(ctx, config); err != nil {
		return fmt.Errorf("provision start: %w", err)
	}
	return nil
}

func loadConfig(path string) (*datamodel.NodeBootstrappingConfiguration, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}
	switch config.Version {
	case ConfigVersion:
		nbc := &datamodel.NodeBootstrappingConfiguration{}
		if err := json.Unmarshal(content, nbc); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %w", err)
		}
		return nbc, nil
	case "":
		return nil, fmt.Errorf("missing config version")
	default:
		return nil, fmt.Errorf("unsupported config version: %s", config.Version)
	}
}

func provisionStart(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) error {
	// CSEScript can't be logged because it contains sensitive information.
	slog.Info("Running CSE script")
	cse, err := CSEScript(ctx, config)
	if err != nil {
		return fmt.Errorf("preparing CSE: %w", err)
	}
	// TODO: add Windows support
	//nolint:gosec // we generate the script, so it's safe to execute
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", cse.UnsafeValue())
	cmd.Dir = "/"
	var stdoutBuf, stderrBuf bytes.Buffer
	// We want to preserve the original stdout and stderr to avoid any issues during migration to the "scriptless" approach
	// RP may rely on stdout and stderr for error handling
	// it's also nice to have a single log file for all the important information, so write to both places
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	err = cmd.Run()
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	// Is it ok to log a single line? Is it too much?
	slog.Info("CSE finished", "exitCode", exitCode, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "error", err)
	return err
}

func CSEScript(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) (SensitiveString, error) {
	ab, err := agent.NewAgentBaker()
	if err != nil {
		return "", err
	}

	nodeBootstrapping, err := ab.GetNodeBootstrapping(ctx, config)
	if err != nil {
		return "", err
	}
	return SensitiveString(nodeBootstrapping.CSE), nil
}

// re-implement CustomData + cloud-init logic from AgentBaker
// only for files not copied during build process.
func writeCustomData(config *datamodel.NodeBootstrappingConfiguration) error {
	files, err := customData(config)
	if err != nil {
		return err
	}
	for path, file := range files {
		slog.Info(fmt.Sprintf("Saving file %s ", path))

		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
		if err := os.WriteFile(path, []byte(file.Content), file.Mode); err != nil {
			return fmt.Errorf("write file %s: %w", path, err)
		}
	}
	return nil
}

type File struct {
	Content string
	Mode    os.FileMode
}

func customData(config *datamodel.NodeBootstrappingConfiguration) (map[string]File, error) {
	contentDockerDaemon, err := generateContentDockerDaemonJSON(config)
	if err != nil {
		return nil, fmt.Errorf("content docker daemon json: %w", err)
	}

	files := map[string]File{
		"/etc/kubernetes/certs/ca.crt": {
			Content: config.ContainerService.Properties.CertificateProfile.CaCertificate,
			Mode:    ReadOnlyUser,
		},
		"/etc/systemd/system/docker.service.d/exec_start.conf": {
			Content: generateContentDockerExecStart(config),
			Mode:    ReadOnlyWorld,
		},
		"/etc/docker/daemon.json": {
			Content: contentDockerDaemon,
			Mode:    ReadOnlyWorld,
		},
		"/etc/default/kubelet": {
			Content: generateContentKubelet(config),
			Mode:    ReadOnlyWorld,
		},
	}

	if config.ContainerService.Properties.SecurityProfile.GetPrivateEgressContainerRegistryServer() != "" {
		files["/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"] = File{
			Content: containerDMCRHosts(config),
			Mode:    ReadOnlyWorld,
		}
	}

	if config.EnableSecureTLSBootstrapping || agent.IsTLSBootstrappingEnabledWithHardCodedToken(config.KubeletClientTLSBootstrapToken) {
		if err := useBootstrappingKubeConfig(config, files); err != nil {
			return nil, err
		}
	} else {
		useHardCodedKubeconfig(config, files)
	}

	for path, file := range files {
		file.Content = strings.TrimLeft(file.Content, "\n")
		files[path] = file
	}

	return files, nil
}

func useHardCodedKubeconfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) {
	files["/var/lib/kubelet/kubeconfig"] = File{
		Content: generateContentKubeconfig(config),
		Mode:    ReadOnlyWorld,
	}
}

func useBootstrappingKubeConfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	bootstrapKubeconfig, err := generateContentBootstrapKubeconfig(config)
	if err != nil {
		return fmt.Errorf("content bootstrap kubeconfig: %w", err)
	}
	files["/var/lib/kubelet/bootstrap-kubeconfig"] = File{
		Content: bootstrapKubeconfig,
		Mode:    ReadOnlyWorld,
	}
	return nil
}

func generateContentKubeconfig(config *datamodel.NodeBootstrappingConfiguration) string {
	users := `- name: client
  user:
    client-certificate: /etc/kubernetes/certs/client.crt
    client-key: /etc/kubernetes/certs/client.key`

	return fmt.Sprintf(`
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://%s:443
users:
%s
contexts:
- context:
    cluster: localcluster
    user: client
  name: localclustercontext
current-context: localclustercontext
`, agent.GetKubernetesEndpoint(config.ContainerService), users)
}

func generateContentBootstrapKubeconfig(config *datamodel.NodeBootstrappingConfiguration) (string, error) {
	data := map[string]any{
		"apiVersion": "v1",
		"kind":       "Config",
		"clusters": []map[string]any{
			{
				"name": "localcluster",
				"cluster": map[string]any{
					"certificate-authority": "/etc/kubernetes/certs/ca.crt",
					"server":                "https://" + agent.GetKubernetesEndpoint(config.ContainerService) + ":443",
				},
			},
		},
		"users": []map[string]any{
			{
				"name": "kubelet-bootstrap",
				"user": func() map[string]any {
					if config.EnableSecureTLSBootstrapping {
						appID := config.CustomSecureTLSBootstrapAADServerAppID
						if appID == "" {
							appID = DefaultAksAadAppID
						}
						return map[string]any{
							"exec": map[string]any{
								"apiVersion": "client.authentication.k8s.io/v1",
								"command":    "/opt/azure/tlsbootstrap/tls-bootstrap-client",
								"args": []string{
									"bootstrap",
									"--next-proto=aks-tls-bootstrap",
									"--aad-resource=" + appID},
								"interactiveMode":    "Never",
								"provideClusterInfo": true,
							},
						}
					}
					return map[string]any{
						"token": agent.GetTLSBootstrapTokenForKubeConfig(config.KubeletClientTLSBootstrapToken),
					}
				}(),
			},
		},
		"contexts": []map[string]any{
			{
				"context": map[string]any{
					"cluster": "localcluster",
					"user":    "kubelet-bootstrap",
				},
				"name": "bootstrap-context",
			},
		},
		"current-context": "bootstrap-context",
	}
	dataYAML, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(dataYAML), nil
}

func generateContentDockerExecStart(config *datamodel.NodeBootstrappingConfiguration) string {
	return fmt.Sprintf(`
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// --storage-driver=overlay2 --bip=%s
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
#EOF`, config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.DockerBridgeSubnet)
}

func generateContentDockerDaemonJSON(config *datamodel.NodeBootstrappingConfiguration) (string, error) {
	data := map[string]any{
		"live-restore": true,
		"log-driver":   "json-file",
		"log-opts": map[string]string{
			"max-size": "50m",
			"max-file": "5",
		},
	}
	if config.EnableNvidia {
		data["default-runtime"] = "nvidia"
		data["runtimes"] = map[string]any{
			"nvidia": map[string]any{
				"path":        "/usr/bin/nvidia-container-runtime",
				"runtimeArgs": []string{},
			},
		}
	}
	if agent.HasDataDir(config) {
		data["data-root"] = agent.GetDataDir(config)
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(dataJSON), nil
}

func generateContentKubelet(config *datamodel.NodeBootstrappingConfiguration) string {
	data := make([][2]string, 0)
	data = append(data, [2]string{"KUBELET_FLAGS", agent.GetOrderedKubeletConfigFlagString(config)})
	data = append(data, [2]string{"KUBELET_REGISTER_SCHEDULABLE", "true"})
	data = append(data, [2]string{"NETWORK_POLICY", config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy})
	isKubernetesVersionGe := func(version string) bool {
		isKubernetes := config.ContainerService.Properties.OrchestratorProfile.IsKubernetes()
		isKubernetesVersionGe := agent.IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, version)
		return isKubernetes && isKubernetesVersionGe
	}

	if !isKubernetesVersionGe("1.17.0") {
		data = append(data, [2]string{"KUBELET_IMAGE", config.K8sComponents.HyperkubeImageURL})
	}

	labels := func() string {
		if isKubernetesVersionGe("1.16.0") {
			return agent.GetAgentKubernetesLabels(config.AgentPoolProfile, config)
		}
		return config.AgentPoolProfile.GetKubernetesLabels()
	}

	data = append(data, [2]string{"KUBELET_NODE_LABELS", labels()})
	if config.ContainerService.IsAKSCustomCloud() {
		data = append(data, [2]string{"AZURE_ENVIRONMENT_FILEPATH", "/etc/kubernetes/" + config.ContainerService.Properties.CustomCloudEnv.Name + ".json"})
	}

	result := ""
	for _, d := range data {
		result += fmt.Sprintf("%s=%s\n", d[0], d[1])
	}
	return result
}

func containerDMCRHosts(config *datamodel.NodeBootstrappingConfiguration) string {
	return fmt.Sprintf(`
[host."https://%s"]
capabilities = ["pull", "resolve"]
`, config.ContainerService.Properties.SecurityProfile.GetPrivateEgressContainerRegistryServer())
}

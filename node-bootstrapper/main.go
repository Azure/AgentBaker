package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
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

	if err := writeCustomData(ctx, config); err != nil {
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

	var cmd *exec.Cmd
	if config.AgentPoolProfile.IsWindows() {
		systemDrive := os.Getenv("SYSTEMDRIVE")
		slog.Info(fmt.Sprintf("systemDrive: %s\n\n", systemDrive))
		if systemDrive == "" {
			systemDrive = "C:"
		}
		script := cse
		script = strings.ReplaceAll(script, "%SYSTEMDRIVE%", systemDrive)
		script = strings.ReplaceAll(script, "\"", "")
		script, found := strings.CutPrefix(script, "powershell.exe -ExecutionPolicy Unrestricted -command ")
		if !found {
			return fmt.Errorf("expected windows script prefix not found: %w", err)
		}
		slog.Info(fmt.Sprintf("CSE script: %s\n\n", script))
		//nolint:gosec // we generate the script, so it's safe to execute
		cmd = exec.CommandContext(ctx, "powershell.exe", "-ExecutionPolicy", "Unrestricted", "-command", script)
	} else {
		//nolint:gosec // we generate the script, so it's safe to execute
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", cse)
	}
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

func OldCustomData(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) (string, error) {
	ab, err := agent.NewAgentBaker()
	if err != nil {
		return "", err
	}

	nodeBootstrapping, err := ab.GetNodeBootstrapping(ctx, config)
	if err != nil {
		return "", err
	}
	return nodeBootstrapping.CustomData, nil
}

// re-implement CustomData + cloud-init logic from AgentBaker
// only for files not copied during build process.
func writeCustomData(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) error {
	files, err := customData(ctx, config)
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

func customData(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) (map[string]File, error) {
	if config.AgentPoolProfile.IsWindows() {
		customData, err2 := OldCustomData(ctx, config)
		if err2 != nil {
			log.Fatal("error:", err2)
		}
		customDataDecoded, err2 := base64.StdEncoding.DecodeString(customData)
		if err2 != nil {
			return nil, err2
		}
		files := map[string]File{"/AzureData/CustomData.bin": File{
			Content: string(customDataDecoded),
			Mode:    ReadOnlyWorld,
		}}
		err := useKubeconfig(config, files)
		if err != nil {
			return nil, err
		}
		return files, nil
	}

	contentDockerDaemon, err := contentDockerDaemonJSON(config)
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

	err2 := useKubeconfig(config, files)
	if err2 != nil {
		return nil, err2
	}

	for path, file := range files {
		file.Content = strings.TrimLeft(file.Content, "\n")
		files[path] = file
	}

	return files, nil
}

func useKubeconfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	switch config.BootstrappingMethod {
	case datamodel.UseArcMsiToMakeCSR:
		if err2 := useBootstrappingKubeConfig(config, files); err2 != nil {
			return err2
		}
		if err2 := useArcTokenSh(config, files); err2 != nil {
			return err2
		}

	case datamodel.UseArcMsiDirectly:
		if err2 := useHardCodedKubeconfig(config, files); err2 != nil {
			return err2
		}
		if err2 := useArcTokenSh(config, files); err2 != nil {
			return err2
		}

	case datamodel.UseAzureMsiDirectly:
		if err2 := useHardCodedKubeconfig(config, files); err2 != nil {
			return err2
		}
		if err2 := useAzureTokenSh(config, files); err2 != nil {
			return err2
		}

	case datamodel.UseAzureMsiToMakeCSR:
		if err2 := useBootstrappingKubeConfig(config, files); err2 != nil {
			return err2
		}
		if err2 := useAzureTokenSh(config, files); err2 != nil {
			return err2
		}

	case datamodel.UseTlsBootstrapToken, datamodel.UseSecureTlsBootstrapping:
		if err2 := useBootstrappingKubeConfig(config, files); err2 != nil {
			return err2
		}

	default:
		if config.EnableSecureTLSBootstrapping || agent.IsTLSBootstrappingEnabledWithHardCodedToken(config.KubeletClientTLSBootstrapToken) {
			if err2 := useBootstrappingKubeConfig(config, files); err2 != nil {
				return err2
			}
		} else {
			if err2 := useHardCodedKubeconfig(config, files); err2 != nil {
				return err2
			}
		}
	}
	return nil
}

func getBootstrapKubeconfigPath(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return "c:\\\\k\\bootstrap-config"
	}
	return "/var/lib/kubelet/bootstrap-kubeconfig"
}

func getHardCodedKubeconfigPath(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return "c:\\\\k\\config"
	}
	return "/var/lib/kubelet/kubeconfig"
}

func getArcTokenPath(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return "c:\\\\k\\arc-token.sh"
	}
	return "/opt/azure/bootstrap/arc-token.sh"
}

func getAzureTokenPath(config *datamodel.NodeBootstrappingConfiguration) string {
	if config.AgentPoolProfile.IsWindows() {
		return "c:\\\\k\\azure-token.sh"
	}
	return "/opt/azure/bootstrap/azure-token.sh"
}

func useHardCodedKubeconfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	files[getHardCodedKubeconfigPath(config)] = File{
		Content: generateContentKubeconfig(config),
		Mode:    ReadOnlyWorld,
	}
	return nil
}

func useArcTokenSh(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	bootstrapKubeconfig := contentArcTokenSh(config)
	files[getArcTokenPath(config)] = File{
		Content: bootstrapKubeconfig,
		Mode:    0755,
	}
	return nil
}

func useAzureTokenSh(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	bootstrapKubeconfig := contentAzureTokenSh(config)
	files[getAzureTokenPath(config)] = File{
		Content: bootstrapKubeconfig,
		Mode:    0755,
	}
	return nil
}

func useBootstrappingKubeConfig(config *datamodel.NodeBootstrappingConfiguration, files map[string]File) error {
	bootstrapKubeconfig, err := generateContentBootstrapKubeconfig(config)
	if err != nil {
		return fmt.Errorf("content bootstrap kubeconfig: %w", err)
	}

	files[getBootstrapKubeconfigPath(config)] = File{
		Content: bootstrapKubeconfig,
		Mode:    0644,
	}
	return nil
}

func generateContentKubeconfig(config *datamodel.NodeBootstrappingConfiguration) string {
	var users string
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}

	switch config.BootstrappingMethod {
	case datamodel.UseArcMsiDirectly:
		users = fmt.Sprintf(`- name: default-auth
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: %s
      provideClusterInfo: false
`, getArcTokenPath(config))

	case datamodel.UseAzureMsiDirectly:
		if config.AgentPoolProfile.IsWindows() {
			users = fmt.Sprintf(`- name: default-auth
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: kubelogin
      args:
      - get-token
      - --environment
      - AzurePublicCloud
      - --server-id
      - %s
      - --login
      - msi
      - --client-id
      - "5f0b9406-fbf1-4e1c-8a61-b6f4a6702057"
      provideClusterInfo: false
`, appID)

		} else {
			users = fmt.Sprintf(`- name: default-auth
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1
      command: %s
      provideClusterInfo: false
`, getAzureTokenPath(config))
		}
	default:
		users = `- name: client
  user:
    client-certificate: /etc/kubernetes/certs/client.crt
    client-key: /etc/kubernetes/certs/client.key`
	}

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

func generateContentArcTokenSh(config *datamodel.NodeBootstrappingConfiguration) string {
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}

	return fmt.Sprintf(`#!/bin/bash

# Fetch an AAD token from Azure Arc HIMDS and output it in the ExecCredential format
# https://learn.microsoft.com/azure/azure-arc/servers/managed-identity-authentication

TOKEN_URL="http://127.0.0.1:40342/metadata/identity/oauth2/token?api-version=2019-11-01&resource=%s"
EXECCREDENTIAL='''
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1",
  "spec": {
    "interactive": false
  },
  "status": {
    "expirationTimestamp": .expires_on | tonumber | todate,
    "token": .access_token
  }
}
'''

# Arc IMDS requires a challenge token from a file only readable by root for security
CHALLENGE_TOKEN_PATH=$(curl -s -D - -H Metadata:true $TOKEN_URL | grep Www-Authenticate | cut -d "=" -f 2 | tr -d "[:cntrl:]")
CHALLENGE_TOKEN=$(cat $CHALLENGE_TOKEN_PATH)
if [ $? -ne 0 ]; then
    echo "Could not retrieve challenge token, double check that this command is run with root privileges."
    exit 255
fi

curl -s -H Metadata:true -H "Authorization: Basic $CHALLENGE_TOKEN" $TOKEN_URL | jq "$EXECCREDENTIAL"
`, appID)
}

func generateContentAzureTokenSh(config *datamodel.NodeBootstrappingConfiguration) string {
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}

	return fmt.Sprintf(`#!/bin/bash

TOKEN_URL="http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=%s"
EXECCREDENTIAL='''
{
  "kind": "ExecCredential",
  "apiVersion": "client.authentication.k8s.io/v1",
  "spec": {
    "interactive": false
  },
  "status": {
    "expirationTimestamp": .expires_on | tonumber | todate,
    "token": .access_token
  }
}
'''

curl -s -H Metadata:true $TOKEN_URL | jq "$EXECCREDENTIAL"
`, appID)
}

func generateContentBootstrapKubeconfig(config *datamodel.NodeBootstrappingConfiguration) (string, error) {
	appID := config.CustomSecureTLSBootstrapAADServerAppID
	if appID == "" {
		appID = DefaultAksAadAppID
	}
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
					switch config.BootstrappingMethod {
					case datamodel.UseArcMsiToMakeCSR:
						return map[string]any{
							"exec": map[string]any{
								"apiVersion":         "client.authentication.k8s.io/v1",
								"command":            getArcTokenPath(config),
								"interactiveMode":    "Never",
								"provideClusterInfo": false,
							},
						}

					case datamodel.UseAzureMsiToMakeCSR:
						if config.AgentPoolProfile.IsWindows() {
							return map[string]any{
								"exec": map[string]any{
									"apiVersion":         "client.authentication.k8s.io/v1",
									"command":            "kubelogin",
									"interactiveMode":    "Never",
									"provideClusterInfo": false,
									"args": []string{
										"get-token",
										"--environment",
										"AzurePublicCloud",
										"--server-id",
										appID,
										"--login",
										"msi",
										"--client-id",
										// hard coded at the moment for Singularity team.
										"5f0b9406-fbf1-4e1c-8a61-b6f4a6702057",
									},
								},
							}
						} else {
							return map[string]any{
								"exec": map[string]any{
									"apiVersion":         "client.authentication.k8s.io/v1",
									"command":            getAzureTokenPath(config),
									"interactiveMode":    "Never",
									"provideClusterInfo": false,
								},
							}
						}
					}
					if config.EnableSecureTLSBootstrapping || config.BootstrappingMethod == datamodel.UseSecureTlsBootstrapping {
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

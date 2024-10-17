package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// Some options are intentionally non-configurable to avoid customization by users
// it will help us to avoid introducing any breaking changes in the future.
const (
	ConfigVersion                  = "v0"
	DefaultAksAadAppID             = "6dae42f8-4368-4678-94ff-3960e28e3630"
	LinuxLogFilePath               = "/var/log/azure"
	WindowsLogFilePath             = "c:\\k"
	LogFileName                    = "node-bootstrapper.log"
	ReadOnlyUser       os.FileMode = 0600
	ReadOnlyWorld      os.FileMode = 0644
	ExecutableWorld    os.FileMode = 0755
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

func isWindows() bool {
	return os.PathSeparator == '\\' && os.PathListSeparator == ';'
}

func mainWithDefer(logFile *os.File) error {
	defer logFile.Close()

	logger := slog.New(slog.NewJSONHandler(logFile, nil))
	slog.SetDefault(logger)

	slog.Info("node-bootstrapper started")
	ctx := context.Background()
	return Run(ctx)
}

func main() {
	logFile, err := getLogFile()
	if err != nil {
		//nolint:forbidigo // there is no other way to communicate the error
		fmt.Printf("failed to open log file: %s\n", err)
		os.Exit(1)
	}

	// mainWithDefer is responsible for closing the log file. That way we know it's closed
	// and can use os.Exit safely in this method.
	err = mainWithDefer(logFile)

	if err != nil {
		slog.Error("node-bootstrapper finished with error", "error", err.Error())
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	slog.Info("node-bootstrapper finished")
}

func getLogFile() (*os.File, error) {
	logFilePath := LinuxLogFilePath
	if isWindows() {
		logFilePath = WindowsLogFilePath
	}

	err := os.MkdirAll(logFilePath, ExecutableWorld)
	if err != nil {
		return nil, err
	}

	logFile := filepath.Join(logFilePath, LogFileName)

	return os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, ReadOnlyWorld)
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
		script := string(cse)
		script = strings.ReplaceAll(script, "%SYSTEMDRIVE%", systemDrive)
		script = strings.ReplaceAll(script, "\"", "")
		script, found := strings.CutPrefix(script, "powershell.exe -ExecutionPolicy Unrestricted -command ")
		if !found {
			return fmt.Errorf("expected windows script prefix not found: %w", err)
		}
		slog.Info(fmt.Sprintf("CSE script: %s\n\n", script))
		cmd = exec.CommandContext(ctx, "powershell.exe", "-ExecutionPolicy", "Unrestricted", "-command", script)
	} else {
		//nolint:gosec // we generate the script, so it's safe to execute
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", string(cse))
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

	contentDockerDaemon, err := genContentDockerDaemonJSON(config)
	if err != nil {
		return nil, fmt.Errorf("content docker daemon json: %w", err)
	}

	files := map[string]File{
		"/etc/kubernetes/certs/ca.crt": {
			Content: config.ContainerService.Properties.CertificateProfile.CaCertificate,
			Mode:    ReadOnlyUser,
		},
		"/etc/systemd/system/docker.service.d/exec_start.conf": {
			Content: genContentDockerExecStart(config),
			Mode:    ReadOnlyWorld,
		},
		"/etc/docker/daemon.json": {
			Content: contentDockerDaemon,
			Mode:    ReadOnlyWorld,
		},
		"/etc/default/kubelet": {
			Content: genContentKubelet(config),
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

func genContentDockerExecStart(config *datamodel.NodeBootstrappingConfiguration) string {
	return fmt.Sprintf(`
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// --storage-driver=overlay2 --bip=%s
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
#EOF`, config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.DockerBridgeSubnet)
}

func genContentDockerDaemonJSON(config *datamodel.NodeBootstrappingConfiguration) (string, error) {
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

func containerDMCRHosts(config *datamodel.NodeBootstrappingConfiguration) string {
	return fmt.Sprintf(`
[host."https://%s"]
capabilities = ["pull", "resolve"]
`, config.ContainerService.Properties.SecurityProfile.GetPrivateEgressContainerRegistryServer())
}

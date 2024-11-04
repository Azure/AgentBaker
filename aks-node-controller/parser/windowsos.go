package parser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	aksnodeconfigv1 "github.com/Azure/agentbaker/pkg/proto/aksnodeconfig/v1"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	windowsLogFilePath             = "c:\\k"
	ReadOnlyUser       os.FileMode = 0600
	ReadOnlyWorld      os.FileMode = 0644
	ExecutableWorld    os.FileMode = 0755
)

type windowsOperatingSystemInfo struct {
}

func (a *windowsOperatingSystemInfo) LogFilePath() string {
	return windowsLogFilePath
}

func (a *windowsOperatingSystemInfo) BuildCSECmd(ctx context.Context, config *aksnodeconfigv1.Configuration) (*exec.Cmd, error) {
	cse, err := CSEScript(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("preparing CSE: %w", err)
	}
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
		return nil, fmt.Errorf("expected windows script prefix not found: %w", err)
	}
	slog.Info(fmt.Sprintf("CSE script: %s\n\n", script))
	cmd := exec.CommandContext(ctx, "powershell.exe", "-ExecutionPolicy", "Unrestricted", "-command", script)

	env := mapToEnviron(getCSEEnv(config))
	cmd.Env = append(os.Environ(), env...) // append existing environment variables
	sort.Strings(cmd.Env)

	return cmd, nil
}

func CSEScript(ctx context.Context, config *aksnodeconfigv1.Configuration) (string, error) {
	ab, err := agent.NewAgentBaker()
	if err != nil {
		return "", err
	}

	nodeBootstrapping, err := ab.GetNodeBootstrapping(ctx, config)
	if err != nil {
		return "", err
	}
	return nodeBootstrapping.CSE, nil
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

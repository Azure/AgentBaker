package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	yaml "sigs.k8s.io/yaml/goyaml.v3" // TODO: should we use JSON instead of YAML to avoid 3rd party dependencies?
)

func main() {
	slog.Info("node-bootstrapper started")
	ctx := context.Background()
	if err := run(ctx); err != nil {
		slog.Error("node-bootstrapper finished with error", "error", err.Error())
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	slog.Info("node-bootstrapper finished")
}

func run(ctx context.Context) error {
	config, err := loadConfig("config.json")
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
	config := &datamodel.NodeBootstrappingConfiguration{}
	configFile, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}
	return config, nil
}

func provisionStart(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) error {
	// CSEScript can't be logged because it contains sensitive information
	slog.Info("Running CSE script")
	defer slog.Info("CSE script finished")
	cse, err := CSEScript(ctx, config)
	if err != nil {
		return fmt.Errorf("cse script: %w", err)
	}

	// TODO: add Windows support
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", cse)
	cmd.Dir = "/"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func CSEScript(ctx context.Context, config *datamodel.NodeBootstrappingConfiguration) (string, error) {
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

func writeCustomData(config *datamodel.NodeBootstrappingConfiguration) error {
	files, err := customData(config)
	if err != nil {
		return err
	}
	for path, file := range files {
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
	contentDockerDaemon, err := contentDockerDaemonJSON(config)
	if err != nil {
		return nil, fmt.Errorf("content docker daemon json: %w", err)
	}

	files := map[string]File{
		"/etc/kubernetes/certs/ca.crt": {
			Content: config.ContainerService.Properties.CertificateProfile.CaCertificate,
			Mode:    0600,
		},
		"/etc/systemd/system/docker.service.d/exec_start.conf": {
			Content: contentDockerExecStart(config),
			Mode:    0644,
		},
		"/etc/docker/daemon.json": {
			Content: contentDockerDaemon,
			Mode:    0644,
		},
		"/etc/default/kubelet": {
			Content: contentKubelet(config),
			Mode:    0644,
		},
	}

	if config.ContainerService.Properties.SecurityProfile.GetPrivateEgressContainerRegistryServer() != "" {
		files["/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml"] = File{
			Content: containerDMCRHosts(config),
			Mode:    0644,
		}
	}

	if config.EnableSecureTLSBootstrapping || agent.IsTLSBootstrappingEnabledWithHardCodedToken(config.KubeletClientTLSBootstrapToken) {
		bootstrapKubeconfig, err := contentBootstrapKubeconfig(config)
		if err != nil {
			return nil, fmt.Errorf("content bootstrap kubeconfig: %w", err)
		}
		files["/var/lib/kubelet/bootstrap-kubeconfig"] = File{
			Content: bootstrapKubeconfig,
			Mode:    0644,
		}
	} else {
		files["/var/lib/kubelet/kubeconfig"] = File{
			Content: contentKubeconfig(config),
			Mode:    0644,
		}
	}

	for path, file := range files {
		file.Content = strings.TrimLeft(file.Content, "\n")
		files[path] = file
	}

	return files, nil
}

func contentKubeconfig(config *datamodel.NodeBootstrappingConfiguration) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://%s:443
users:
- name: client
  user:
    client-certificate: /etc/kubernetes/certs/client.crt
    client-key: /etc/kubernetes/certs/client.key
contexts:
- context:
    cluster: localcluster
    user: client
  name: localclustercontext
current-context: localclustercontext
`, agent.GetKubernetesEndpoint(config.ContainerService))
}

func contentBootstrapKubeconfig(config *datamodel.NodeBootstrappingConfiguration) (string, error) {
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
							appID = "6dae42f8-4368-4678-94ff-3960e28e3630"
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

func contentDockerExecStart(config *datamodel.NodeBootstrappingConfiguration) string {
	return fmt.Sprintf(`
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// --storage-driver=overlay2 --bip=%s
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
#EOF`, config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.DockerBridgeSubnet)
}

func contentDockerDaemonJSON(config *datamodel.NodeBootstrappingConfiguration) (string, error) {
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

func contentKubelet(config *datamodel.NodeBootstrappingConfiguration) string {
	data := make([][2]string, 0)
	data = append(data, [2]string{"KUBELET_FLAGS", agent.GetOrderedKubeletConfigFlagString(config)})
	data = append(data, [2]string{"KUBELET_REGISTER_SCHEDULABLE", "true"})
	data = append(data, [2]string{"NETWORK_POLICY", config.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.NetworkPolicy})
	IsKubernetesVersionGe := func(version string) bool {
		return config.ContainerService.Properties.OrchestratorProfile.IsKubernetes() && agent.IsKubernetesVersionGe(config.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion, version)
	}

	if !IsKubernetesVersionGe("1.17.0") {
		data = append(data, [2]string{"KUBELET_IMAGE", config.K8sComponents.HyperkubeImageURL})
	}

	labels := func() string {
		if IsKubernetesVersionGe("1.16.0") {
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

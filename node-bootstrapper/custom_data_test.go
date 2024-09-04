package main

import (
	"io/fs"
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomData(t *testing.T) {
	getFile := func(t *testing.T, nbc *datamodel.NodeBootstrappingConfiguration, path string, expectedMode fs.FileMode) string {
		t.Helper()
		files, err := customData(nbc)
		require.NoError(t, err)
		require.Contains(t, files, path)
		actual := files[path]
		assert.Equal(t, expectedMode, actual.Mode)
		return actual.Content
	}

	t.Run("kubeconfig", func(t *testing.T) {
		nbc := validNBC()
		actual := getFile(t, nbc, "/var/lib/kubelet/kubeconfig", 0644)
		expected := `
apiVersion: v1
kind: Config
clusters:
- name: localcluster
  cluster:
    certificate-authority: /etc/kubernetes/certs/ca.crt
    server: https://:443
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
`
		assert.YAMLEq(t, expected, actual)
	})

	t.Run("ca.crt", func(t *testing.T) {
		nbc := validNBC()
		actual := getFile(t, nbc, "/etc/kubernetes/certs/ca.crt", 0600)
		expected := "test-ca-cert"
		assert.Equal(t, expected, actual)
	})

	t.Run("bootstrap-kubeconfig", func(t *testing.T) {
		nbc := validNBC()
		nbc.KubeletClientTLSBootstrapToken = Ptr("test-token")
		actual := getFile(t, nbc, "/var/lib/kubelet/bootstrap-kubeconfig", 0644)
		expected := `apiVersion: v1
clusters:
    - cluster:
        certificate-authority: /etc/kubernetes/certs/ca.crt
        server: https://:443
      name: localcluster
contexts:
    - context:
        cluster: localcluster
        user: kubelet-bootstrap
      name: bootstrap-context
current-context: bootstrap-context
kind: Config
users:
    - name: kubelet-bootstrap
      user:
        token: test-token
`
		assert.YAMLEq(t, expected, actual)
	})

	t.Run("exec_start.conf", func(t *testing.T) {
		nbc := validNBC()
		actual := getFile(t, nbc, "/etc/systemd/system/docker.service.d/exec_start.conf", 0644)
		nbc.ContainerService.Properties.OrchestratorProfile.KubernetesConfig.DockerBridgeSubnet = "1.1.1.1"
		expected := `[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// --storage-driver=overlay2 --bip=1.1.1.1
ExecStartPost=/sbin/iptables -P FORWARD ACCEPT
#EOF`
		assert.Equal(t, expected, actual)
	})

	t.Run("docker-daemon.json", func(t *testing.T) {
		nbc := validNBC()
		actual := getFile(t, nbc, "/etc/docker/daemon.json", 0644)
		expected := `
{
	"data-root":"/mnt/aks/containers",
	"live-restore":true,
	"log-driver":"json-file",
	"log-opts": {
		"max-file":"5",
		"max-size":"50m"
	}
}
`
		assert.JSONEq(t, expected, actual)
	})
	t.Run("kubelet", func(t *testing.T) {
		nbc := validNBC()
		actual := getFile(t, nbc, "/etc/default/kubelet", 0644)
		expected := `KUBELET_FLAGS=
KUBELET_REGISTER_SCHEDULABLE=true
NETWORK_POLICY=
KUBELET_NODE_LABELS=agentpool=,kubernetes.azure.com/agentpool=
`
		assert.Equal(t, expected, actual)
	})

	t.Run("containerDMCRHosts", func(t *testing.T) {
		nbc := validNBC()
		nbc.ContainerService.Properties.SecurityProfile = &datamodel.SecurityProfile{
			PrivateEgress: &datamodel.PrivateEgress{
				Enabled:                 true,
				ContainerRegistryServer: "test-registry",
			},
		}
		actual := getFile(t, nbc, "/etc/containerd/certs.d/mcr.microsoft.com/hosts.toml", 0644)
		expected := `[host."https://test-registry"]
capabilities = ["pull", "resolve"]
`
		assert.Equal(t, expected, actual)
	})
}

func validNBC() *datamodel.NodeBootstrappingConfiguration {
	return &datamodel.NodeBootstrappingConfiguration{
		ContainerService: &datamodel.ContainerService{
			Properties: &datamodel.Properties{
				CertificateProfile: &datamodel.CertificateProfile{
					CaCertificate: "test-ca-cert",
				},
				OrchestratorProfile: &datamodel.OrchestratorProfile{
					OrchestratorType:    datamodel.Kubernetes,
					OrchestratorVersion: "1.31.0",
					KubernetesConfig: &datamodel.KubernetesConfig{
						DockerBridgeSubnet: "1.1.1.1",
					},
				},
			},
		},
		CustomSecureTLSBootstrapAADServerAppID: "test-app-id",
		AgentPoolProfile: &datamodel.AgentPoolProfile{
			KubeletDiskType: datamodel.TempDisk,
		},
	}
}

func Ptr[T any](input T) *T {
	return &input
}

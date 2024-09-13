package main

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/assert"
)

func TestCustomData(t *testing.T) {

	t.Run("ca.crt", func(t *testing.T) {
		nbc := validNBC()
		actual := getFile(t, nbc, "/etc/kubernetes/certs/ca.crt", 0600)
		expected := "test-ca-cert"
		assert.Equal(t, expected, actual)
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

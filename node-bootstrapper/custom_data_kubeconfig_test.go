package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKubeConfigGeneratedCorrectly(t *testing.T) {

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

	t.Run("secureTlsBootstrapKubeConfig sets bootstrap-kubeconfig correctly", func(t *testing.T) {
		nbc := validNBC()
		nbc.EnableSecureTLSBootstrapping = true
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
    exec:
      apiVersion: client.authentication.k8s.io/v1
      args:
        - bootstrap
        - --next-proto=aks-tls-bootstrap
        - --aad-resource=test-app-id
      command: /opt/azure/tlsbootstrap/tls-bootstrap-client
      interactiveMode: Never
      provideClusterInfo: true
`
		assert.YAMLEq(t, expected, actual)
	})

	t.Run("BootstrappingMethod=UseSecureTlsBootstrapping sets bootstrap-kubeconfig correctly", func(t *testing.T) {
		nbc := validNBC()
		nbc.BootstrappingMethod = "UseSecureTlsBootstrapping"
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
    exec:
      apiVersion: client.authentication.k8s.io/v1
      args:
        - bootstrap
        - --next-proto=aks-tls-bootstrap
        - --aad-resource=test-app-id
      command: /opt/azure/tlsbootstrap/tls-bootstrap-client
      interactiveMode: Never
      provideClusterInfo: true
`
		assert.YAMLEq(t, expected, actual)
	})

}

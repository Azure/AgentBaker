package e2e

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
	"time"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ValidatePodRunning(ctx context.Context, s *Scenario, pod *corev1.Pod) {
	kube := s.Runtime.Cluster.Kube
	truncatePodName(s.T, pod)
	start := time.Now()

	s.T.Logf("creating pod %q", pod.Name)
	_, err := kube.Typed.CoreV1().Pods(pod.Namespace).Create(ctx, pod, v1.CreateOptions{})
	require.NoErrorf(s.T, err, "failed to create pod %q", pod.Name)
	s.T.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		err := kube.Typed.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, v1.DeleteOptions{GracePeriodSeconds: to.Ptr(int64(0))})
		if err != nil {
			s.T.Logf("couldn't not delete pod %s: %v", pod.Name, err)
		}
	})

	_, err = kube.WaitUntilPodRunning(ctx, pod.Namespace, "", "metadata.name="+pod.Name)
	if err != nil {
		jsonString, jsonError := json.Marshal(pod)
		if jsonError != nil {
			jsonString = []byte(jsonError.Error())
		}
		require.NoErrorf(s.T, err, "failed to wait for pod %q to be in running state. Pod data: %s", pod.Name, jsonString)
	}

	timeForReady := time.Since(start)
	toolkit.LogDuration(ctx, timeForReady, time.Minute, fmt.Sprintf("Time for pod %q to get ready was %s", pod.Name, timeForReady))
	s.T.Logf("node health validation: test pod %q is running on node %q", pod.Name, s.Runtime.VM.KubeName)
}

func truncatePodName(t testing.TB, pod *corev1.Pod) {
	name := pod.Name
	if len(pod.Name) < 63 {
		return
	}
	pod.Name = pod.Name[:63]
	pod.Name = strings.TrimRight(pod.Name, "-")
	t.Logf("truncated pod name %q to %q", name, pod.Name)
}

func ValidateCommonLinux(ctx context.Context, s *Scenario) {
	ValidateTLSBootstrapping(ctx, s)
	ValidateKubeletServingCertificateRotation(ctx, s)
	ValidateSystemdWatchdogForKubernetes132Plus(ctx, s)
	ValidateSystemdUnitIsNotFailed(ctx, s, "aks-log-collector")
	ValidateLeakedSecrets(ctx, s)
	ValidateIPTablesCompatibleWithCiliumEBPF(ctx, s)

	ValidateSysctlConfig(ctx, s, map[string]string{
		"net.ipv4.tcp_retries2":             "8",
		"net.core.message_burst":            "80",
		"net.core.message_cost":             "40",
		"net.core.somaxconn":                "16384",
		"net.ipv4.tcp_max_syn_backlog":      "16384",
		"net.ipv4.neigh.default.gc_thresh1": "4096",
		"net.ipv4.neigh.default.gc_thresh2": "8192",
		"net.ipv4.neigh.default.gc_thresh3": "16384",
	})
	ValidateDirectoryContent(ctx, s, "/var/log/azure/aks", []string{
		"cluster-provision.log",
		"cluster-provision-cse-output.log",
		"cloud-init-files.paved",
		"vhd-install.complete",
		//"cloud-config.txt", // file with UserData
	})

	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo cat /etc/default/kubelet", 0, "could not read kubelet config")
	require.NotContains(s.T, execResult.stdout.String(), "--dynamic-config-dir", "kubelet flag '--dynamic-config-dir' should not be present in /etc/default/kubelet\nContents:\n%s")

	_ = execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo curl http://168.63.129.16:32526/vmSettings", 0, "curl to wireserver failed")

	execResult = execOnVMForScenarioOnUnprivilegedPod(ctx, s, "curl https://168.63.129.16/machine/?comp=goalstate -H 'x-ms-version: 2015-04-05' -s --connect-timeout 4")
	if !assert.Equal(s.T, "28", execResult.exitCode, "curl to wireserver expected to fail, but it didn't") {
		// Log debug information. This validator seems to be flaky, hard to catch
		s.T.Logf("host IPTABLES: %s", execScriptOnVMForScenario(ctx, s, "sudo iptables -t filter -L FORWARD -v -n --line-numbers").String())
		s.T.FailNow()
	}

	execResult = execOnVMForScenarioOnUnprivilegedPod(ctx, s, "curl http://168.63.129.16:32526/vmSettings --connect-timeout 4")
	if !assert.Equal(s.T, "28", execResult.exitCode, "curl to wireserver port 32526 expected to fail, but it didn't") {
		// Log debug information. This validator seems to be flaky, hard to catch
		s.T.Logf("host IPTABLES: %s", execScriptOnVMForScenario(ctx, s, "sudo iptables -t filter -L FORWARD -v -n --line-numbers").String())
		s.T.FailNow()
	}

	// base NBC templates define a mock service principal profile that we can still use to test
	// the correct bootstrapping logic: https://github.com/Azure/AgentBaker/blob/master/e2e/node_config.go#L438-L441
	if hasServicePrincipalData(s) {
		_ = execScriptOnVMForScenarioValidateExitCode(
			ctx,
			s,
			`sudo test -n "$(sudo cat /etc/kubernetes/azure.json | jq -r '.aadClientId')" && sudo test -n "$(sudo cat /etc/kubernetes/azure.json | jq -r '.aadClientSecret')"`,
			0,
			"AAD client ID and secret should be present in /etc/kubernetes/azure.json")
	}

	// kubeletNodeIPValidator cannot be run on older VHDs with kubelet < 1.29
	if !s.VHD.UnsupportedKubeletNodeIP {
		ValidateKubeletNodeIP(ctx, s)
	}

	// localdns is not supported on scriptless, privatekube and VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached.
	if s.Tags.Scriptless != true && s.VHD != config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg && s.VHD != config.VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached {
		ValidateLocalDNSService(ctx, s, "enabled")
		ValidateLocalDNSResolution(ctx, s, "169.254.10.10")
	}
}

func ValidateSystemdWatchdogForKubernetes132Plus(ctx context.Context, s *Scenario) {
	var k8sVersion string
	if s.Runtime.NBC != nil && s.Runtime.NBC.ContainerService != nil &&
		s.Runtime.NBC.ContainerService.Properties != nil &&
		s.Runtime.NBC.ContainerService.Properties.OrchestratorProfile != nil {
		k8sVersion = s.Runtime.NBC.ContainerService.Properties.OrchestratorProfile.OrchestratorVersion
	} else if s.Runtime.AKSNodeConfig != nil {
		k8sVersion = s.Runtime.AKSNodeConfig.GetKubernetesVersion()
	}

	if k8sVersion != "" && agent.IsKubernetesVersionGe(k8sVersion, "1.32.0") {
		// Validate systemd watchdog is enabled and configured for kubelet
		ValidateSystemdUnitIsRunning(ctx, s, "kubelet.service")
		ValidateFileHasContent(ctx, s, "/etc/systemd/system/kubelet.service.d/10-watchdog.conf", "WatchdogSec=60s")
		ValidateJournalctlOutput(ctx, s, "kubelet.service", "Starting systemd watchdog with interval")
	}
}

func ValidateLeakedSecrets(ctx context.Context, s *Scenario) {
	var secrets map[string]string
	b64Encoded := func(val string) string {
		return base64.StdEncoding.EncodeToString([]byte(val))
	}
	if s.Runtime.NBC != nil {
		secrets = map[string]string{
			"client private key":       b64Encoded(s.Runtime.NBC.ContainerService.Properties.CertificateProfile.ClientPrivateKey),
			"service principal secret": b64Encoded(s.Runtime.NBC.ContainerService.Properties.ServicePrincipalProfile.Secret),
			"bootstrap token":          *s.Runtime.NBC.KubeletClientTLSBootstrapToken,
		}
	} else {
		token := s.Runtime.AKSNodeConfig.BootstrappingConfig.TlsBootstrappingToken
		strToken := ""
		if token != nil {
			strToken = *token
		}
		secrets = map[string]string{
			"client private key":       b64Encoded(s.Runtime.AKSNodeConfig.KubeletConfig.KubeletClientKey),
			"service principal secret": b64Encoded(s.Runtime.AKSNodeConfig.AuthConfig.ServicePrincipalSecret),
			"bootstrap token":          strToken,
		}
	}

	for _, logFile := range []string{"/var/log/azure/cluster-provision.log", "/var/log/azure/aks-node-controller.log"} {
		for _, secretValue := range secrets {
			if secretValue != "" {
				ValidateFileExcludesContent(ctx, s, logFile, secretValue)
			}
		}
	}
}

func ValidateKubeletServingCertificateRotation(ctx context.Context, s *Scenario) {
	if _, ok := s.Runtime.VM.VMSS.Tags["aks-disable-kubelet-serving-certificate-rotation"]; ok {
		s.T.Logf("ValidateKubeletServingCertificateRotation - VMSS has KSCR disablement tag, will validate that KSCR has been disabled")
		ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "--rotate-server-certificates=true")
		ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "kubernetes.azure.com/kubelet-serving-ca=cluster")
		ValidateFileHasContent(ctx, s, "/etc/default/kubelet", "--tls-cert-file")
		ValidateFileHasContent(ctx, s, "/etc/default/kubelet", "--tls-private-key-file")
		ValidateDirectoryContent(ctx, s, "/etc/kubernetes/certs", []string{"kubeletserver.crt", "kubeletserver.key"})
		if hasKubeletConfigFile(s) {
			ValidateFileHasContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsCertFile\": \"/etc/kubernetes/certs/kubeletserver.crt\"")
			ValidateFileHasContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsPrivateKeyFile\": \"/etc/kubernetes/certs/kubeletserver.key\"")
			ValidateFileExcludesContent(ctx, s, "/etc/default/kubeletconfig.json", "\"serverTLSBootstrap\": true")
		}
		return
	}
	ValidateFileHasContent(ctx, s, "/etc/default/kubelet", "--rotate-server-certificates=true")
	ValidateFileHasContent(ctx, s, "/etc/default/kubelet", "kubernetes.azure.com/kubelet-serving-ca=cluster")
	ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "--tls-cert-file")
	ValidateFileExcludesContent(ctx, s, "/etc/default/kubelet", "--tls-private-key-file")
	ValidateDirectoryContent(ctx, s, "/var/lib/kubelet/pki", []string{"kubelet-server-current.pem"})
	if hasKubeletConfigFile(s) {
		ValidateFileExcludesContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsCertFile\": \"/etc/kubernetes/certs/kubeletserver.crt\"")
		ValidateFileExcludesContent(ctx, s, "/etc/default/kubeletconfig.json", "\"tlsPrivateKeyFile\": \"/etc/kubernetes/certs/kubeletserver.key\"")
		ValidateFileHasContent(ctx, s, "/etc/default/kubeletconfig.json", "\"serverTLSBootstrap\": true")
	}
}

func ValidateTLSBootstrapping(ctx context.Context, s *Scenario) {
	ValidateDirectoryContent(ctx, s, "/var/lib/kubelet", []string{"kubeconfig"})
	ValidateDirectoryContent(ctx, s, "/var/lib/kubeket/pki", []string{"kubelet-server-current.pem"})
	kubeletLogs := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo journalctl -u kubelet", 0, "could not retrieve kubelet logs with journalctl").stdout.String()
	switch {
	case isUsingSecureTLSBootstrapping(s) && s.Tags.BootstrapTokenFallback:
		ValidateSystemdUnitIsNotRunning(ctx, s, "secure-tls-bootstrap")
		require.True(
			s.T,
			!strings.Contains(kubeletLogs, "unable to validate bootstrap credentials") && strings.Contains(kubeletLogs, "kubelet bootstrap token credential is valid"),
			"expected to have successfully validated bootstrap token credential before kubelet startup, but did not",
		)
	case isUsingSecureTLSBootstrapping(s):
		ValidateSystemdUnitIsRunning(ctx, s, "secure-tls-bootstrap")
		validateKubeletClientCSRCreatedBySecureTLSBootstrapping(ctx, s)
		require.True(
			s.T,
			!strings.Contains(kubeletLogs, "unable to validate bootstrap credentials") && strings.Contains(kubeletLogs, "client credential already exists within kubeconfig"),
			"expected to already have a valid kubeconfig before kubelet start-up obtained through secure TLS bootstrapping, but did not",
		)
	default:
		ValidateSystemdUnitIsNotRunning(ctx, s, "secure-tls-bootstrap")
		ValidateSystemdUnitIsNotFailed(ctx, s, "secure-tls-bootstrap")
		require.True(
			s.T,
			!strings.Contains(kubeletLogs, "unable to validate bootstrap credentials") && strings.Contains(kubeletLogs, "kubelet bootstrap token credential is valid"),
			"expected to have successfully validated bootstrap token credential before kubelet startup, but did not",
		)
	}
}

func validateKubeletClientCSRCreatedBySecureTLSBootstrapping(ctx context.Context, s *Scenario) {
	kubeletClientCSRs, err := s.Runtime.Cluster.Kube.Typed.CertificatesV1().CertificateSigningRequests().List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.signerName=%s", certv1.KubeAPIServerClientKubeletSignerName),
	})
	require.NoError(s.T, err)
	var hasValidCSR bool
	for _, csr := range kubeletClientCSRs.Items {
		if len(csr.Status.Certificate) == 0 {
			continue
		}
		if strings.HasPrefix(strings.ToLower(csr.Spec.Username), "system:bootstrap:") {
			continue
		}
		if getNodeNameFromCSR(s, csr) == s.Runtime.VM.KubeName {
			hasValidCSR = true
			break
		}
	}
	require.True(s.T, hasValidCSR, "expected node %s to have created a kubelet client CSR which was approved and issued, using secure TLS bootstrapping")
}

func ValidateSSHServiceEnabled(ctx context.Context, s *Scenario) {
	// Verify SSH service is active and running
	ValidateSystemdUnitIsRunning(ctx, s, "ssh")

	// Verify socket-based activation is disabled
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "systemctl is-active ssh.socket", 3, "could not check ssh.socket status")
	stdout := execResult.stdout.String()
	require.Contains(s.T, stdout, "inactive", "ssh.socket should be inactive")

	// Check that systemd recognizes SSH service should be active at boot
	execResult = execScriptOnVMForScenarioValidateExitCode(ctx, s, "systemctl is-enabled ssh.service", 0, "could not check ssh.service status")
	stdout = execResult.stdout.String()
	require.Contains(s.T, stdout, "enabled", "ssh.service should be enabled at boot")
}

func hasServicePrincipalData(s *Scenario) bool {
	if s.Runtime == nil {
		return false
	}
	if s.Runtime.AKSNodeConfig != nil && s.Runtime.AKSNodeConfig.AuthConfig != nil {
		return s.Runtime.AKSNodeConfig.AuthConfig.ServicePrincipalId != "" && s.Runtime.AKSNodeConfig.AuthConfig.ServicePrincipalSecret != ""
	}
	if s.Runtime.NBC != nil && s.Runtime.NBC.ContainerService != nil && s.Runtime.NBC.ContainerService.Properties != nil && s.Runtime.NBC.ContainerService.Properties.ServicePrincipalProfile != nil {
		return s.Runtime.NBC.ContainerService.Properties.ServicePrincipalProfile.ClientID != "" && s.Runtime.NBC.ContainerService.Properties.ServicePrincipalProfile.Secret != ""
	}
	return false
}

func getNodeNameFromCSR(s *Scenario, csr certv1.CertificateSigningRequest) string {
	block, _ := pem.Decode(csr.Spec.Request)
	require.NotNil(s.T, block)
	req, err := x509.ParseCertificateRequest(block.Bytes)
	require.NoError(s.T, err)
	return strings.TrimPrefix(req.Subject.CommonName, "system:node:")
}

func isUsingSecureTLSBootstrapping(s *Scenario) bool {
	return s.Runtime.NBC.SecureTLSBootstrappingConfig.GetEnabled() ||
		s.Runtime.AKSNodeConfig.BootstrappingConfig.GetBootstrappingAuthMethod() == aksnodeconfigv1.BootstrappingAuthMethod_BOOTSTRAPPING_AUTH_METHOD_SECURE_TLS_BOOTSTRAPPING
}

func hasKubeletConfigFile(s *Scenario) bool {
	return s.Runtime.NBC.AgentPoolProfile.CustomKubeletConfig != nil || s.Runtime.AKSNodeConfig.KubeletConfig.EnableKubeletConfigFile
}

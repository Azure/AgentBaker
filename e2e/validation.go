package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo cat /etc/default/kubelet", 0, "could not read kubelet config")
	stdout := execResult.stdout.String()
	require.NotContains(s.T, stdout, "--dynamic-config-dir", "kubelet flag '--dynamic-config-dir' should not be present in /etc/default/kubelet\nContents:\n%s")

	kubeletLogs := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo journalctl -u kubelet", 0, "could not retrieve kubelet logs with journalctl").stdout.String()
	require.True(
		s.T,
		!strings.Contains(kubeletLogs, "unable to validate bootstrap credentials") && strings.Contains(kubeletLogs, "kubelet bootstrap token credential is valid"),
		"expected to have successfully validated bootstrap token credential before kubelet startup, but did not",
	)

	ValidateSystemdWatchdogForKubernetes132Plus(ctx, s)

	// ensure aks-log-collector hasn't entered a failed state
	ValidateSystemdUnitIsNotFailed(ctx, s, "aks-log-collector")

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

	ValidateLeakedSecrets(ctx, s)

	// kubeletNodeIPValidator cannot be run on older VHDs with kubelet < 1.29
	if !s.VHD.UnsupportedKubeletNodeIP {
		ValidateKubeletNodeIP(ctx, s)
	}

	// localdns is not supported on scriptless, privatekube and VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached.
	if s.Tags.Scriptless != true && s.VHD != config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg && s.VHD != config.VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached {
		ValidateLocalDNSService(ctx, s, "enabled")
		ValidateLocalDNSResolution(ctx, s, "169.254.10.10")
	}

	ValidateIPTablesCompatibleWithCiliumEBPF(ctx, s)
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
		s.T.Logf("ValidateKubeletServingCertificateRotation - VMSS has KSCR disablement tag, will skip standard validation")
		return
	}
}

func getCertificateSigningRequestsForNode(ctx context.Context, s *Scenario) (clientCSR *certv1.CertificateSigningRequest, servingCSR *certv1.CertificateSigningRequest, err error) {
	s.T.Logf("attempting to get kubelet client and serving CSRs for node: %s", s.Runtime.VM.KubeName)
	csrClient := s.Runtime.Cluster.Kube.Typed.CertificatesV1().CertificateSigningRequests()

	isApprovedAndIssued := func(csr certv1.CertificateSigningRequest) bool {
		var approved, issued bool
		for _, cond := range csr.Status.Conditions {
			if cond.Type == certv1.CertificateApproved && cond.Status == corev1.ConditionTrue {
				approved = true
			}
		}
		if !approved {
			return false
		}
	}

	servingCSRs, err := csrClient.List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.signerName=%s", certv1.KubeletServingSignerName),
	})
	require.NoError(s.T, err)

	clientCSRs, err := csrClient.List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.signerName=%s", certv1.KubeAPIServerClientKubeletSignerName),
	})
	require.NoError(s.T, err)

	for _, servingCSR := range servingCSRs.Items {

	}

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

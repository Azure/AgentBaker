package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func ValidatePodRunning(ctx context.Context, s *Scenario) {
	testPod := func() *corev1.Pod {
		if s.VHD.OS == config.OSWindows {
			return podHTTPServerWindows(s)
		}
		return podHTTPServerLinux(s)
	}()
	ensurePod(ctx, s, testPod)
	s.T.Logf("node health validation: test pod %q is running on node %q", testPod.Name, s.Runtime.KubeNodeName)
}

func ValidateWASM(ctx context.Context, s *Scenario, nodeName string) {
	s.T.Logf("wasm scenario: running wasm validation on %s...", nodeName)
	spinClassName := fmt.Sprintf("wasmtime-%s", wasmHandlerSpin)
	err := createRuntimeClass(ctx, s.Runtime.Cluster.Kube, spinClassName, wasmHandlerSpin)
	require.NoError(s.T, err)
	err = ensureWasmRuntimeClasses(ctx, s.Runtime.Cluster.Kube)
	require.NoError(s.T, err)
	spinPodManifest := podWASMSpin(s)
	ensurePod(ctx, s, spinPodManifest)
	require.NoError(s.T, err, "unable to ensure wasm pod on node %q", nodeName)
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

	// the instructions belows expects the SSH key to be uploaded to the user pool VM.
	// which happens as a side-effect of execCommandOnVMForScenario, it's ugly but works.
	// maybe we should use a single ssh key per cluster, but need to be careful with parallel test runs.
	logSSHInstructions(s)

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
	require.Equal(s.T, "28", execResult.exitCode, "curl to wireserver should fail")

	execResult = execOnVMForScenarioOnUnprivilegedPod(ctx, s, "curl http://168.63.129.16:32526/vmSettings --connect-timeout 4")
	require.Equal(s.T, "28", execResult.exitCode, "curl to wireserver port 32526 shouldn't succeed")

	if hasServicePrincipalProfile(s) {
		execResult = execScriptOnVMForScenarioValidateExitCode(
			ctx,
			s,
			`test -n "$(jq -r '.aadClientId' < /etc/kubernetes/azure.json)"`,
			0,
			"AAD client ID should be present in /etc/kubernetes/azure.json")
		execResult = execScriptOnVMForScenarioValidateExitCode(
			ctx,
			s,
			`test -n "$(jq -r '.aadClientSecret' < /etc/kubernetes/azure.json)"`,
			0,
			"AAD client ID should be present in /etc/kubernetes/azure.json")
	}

	ValidateLeakedSecrets(ctx, s)

	// kubeletNodeIPValidator cannot be run on older VHDs with kubelet < 1.29
	if s.VHD.Version != config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg.Version {
		ValidateKubeletNodeIP(ctx, s)
	}

	// localdns is not supported on 1804, scriptless, privatekube and VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached.
	if s.Tags.Scriptless != true && s.VHD != config.VHDUbuntu1804Gen2Containerd && s.VHD != config.VHDUbuntu2204Gen2ContainerdPrivateKubePkg && s.VHD != config.VHDUbuntu2204Gen2ContainerdAirgappedK8sNotCached {
		ValidateLocalDNSService(ctx, s)
		ValidateLocalDNSResolution(ctx, s)
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

func hasServicePrincipalProfile(s *Scenario) bool {
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

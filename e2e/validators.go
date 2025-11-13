package e2e

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/blang/semver"
	"github.com/tidwall/gjson"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

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
	switch s.VHD.OS {
	case config.OSWindows:
		validateKubeletServingCertificateRotationWindows(ctx, s)
	default:
		validateKubeletServingCertificateRotationLinux(ctx, s)
	}
}

func validateKubeletServingCertificateRotationLinux(ctx context.Context, s *Scenario) {
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

func validateKubeletServingCertificateRotationWindows(ctx context.Context, s *Scenario) {

}

func ValidateTLSBootstrapping(ctx context.Context, s *Scenario) {
	switch s.VHD.OS {
	case config.OSWindows:
		validateTLSBootstrappingWindows(ctx, s)
	default:
		validateTLSBootstrappingLinux(ctx, s)
	}
}

func validateTLSBootstrappingLinux(ctx context.Context, s *Scenario) {
	ValidateDirectoryContent(ctx, s, "/var/lib/kubelet", []string{"kubeconfig"})
	ValidateDirectoryContent(ctx, s, "/var/lib/kubelet/pki", []string{"kubelet-server-current.pem"})
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

func validateTLSBootstrappingWindows(ctx context.Context, s *Scenario) {

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

func ValidateDirectoryContent(ctx context.Context, s *Scenario, path string, files []string) {
	s.T.Helper()
	steps := []string{
		"set -ex",
		fmt.Sprintf("sudo ls -la %s", path),
	}
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not get directory contents")
	stdout := execResult.stdout.String()
	for _, file := range files {
		require.Contains(s.T, stdout, file, "expected to find file %s within directory %s, but did not.\nDirectory contents:\n%s", file, path, stdout)
	}
}

func ValidateSysctlConfig(ctx context.Context, s *Scenario, customSysctls map[string]string) {
	s.T.Helper()
	keysToCheck := make([]string, len(customSysctls))
	for k := range customSysctls {
		keysToCheck = append(keysToCheck, k)
	}
	command := []string{
		"set -ex",
		fmt.Sprintf("sudo sysctl %s | sed -E 's/([0-9])\\s+([0-9])/\\1 \\2/g'", strings.Join(keysToCheck, " ")),
	}
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "systmctl command failed")
	stdout := execResult.stdout.String()
	for name, value := range customSysctls {
		require.Contains(s.T, stdout, fmt.Sprintf("%s = %v", name, value), "expected to find %s set to %v, but was not.\nStdout:\n%s", name, value, stdout)
	}
}

func ValidateNvidiaSMINotInstalled(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		"sudo nvidia-smi",
	}
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 1, "")
	stderr := execResult.stderr.String()
	require.Contains(s.T, stderr, "nvidia-smi: command not found", "expected stderr to contain 'nvidia-smi: command not found', but got %q", stderr)
}

func ValidateNvidiaSMIInstalled(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{"set -ex", "sudo nvidia-smi"}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "could not execute nvidia-smi command")
}

func ValidateNvidiaModProbeInstalled(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		"sudo nvidia-modprobe",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "could not execute nvidia-modprobe command")
}

func ValidateNvidiaGRIDLicenseValid(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Capture the license status output, or continue silently if not found
		"license_status=$(sudo nvidia-smi -q | grep 'License Status' | grep 'Licensed' || true)",
		// If the output is empty, print an error message and exit with a nonzero code
		"if [ -z \"$license_status\" ]; then echo 'License status not valid or not found'; exit 1; fi",
		// Check that nvidia-gridd is active by capturing its is-active output
		"active_status=$(sudo systemctl is-active nvidia-gridd)",
		"if [ \"$active_status\" != \"active\" ]; then echo \"nvidia-gridd is not active: $active_status\"; exit 1; fi",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to validate nvidia-smi license state or nvidia-gridd service status")
}

func ValidateNvidiaPersistencedRunning(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Check that nvidia-persistenced.service is active by capturing its is-active output
		"active_status=$(sudo systemctl is-active nvidia-persistenced.service)",
		"if [ \"$active_status\" != \"active\" ]; then echo \"nvidia-gridd is not active: $active_status\"; exit 1; fi",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to validate nvidia-persistenced.service status")
}

func ValidateNonEmptyDirectory(ctx context.Context, s *Scenario, dirName string) {
	s.T.Helper()
	command := []string{
		"set -ex",
		fmt.Sprintf("sudo ls -1q %s | grep -q '^.*$' && true || false", dirName),
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "either could not find expected file, or something went wrong")
}

func ValidateFileExists(ctx context.Context, s *Scenario, fileName string) {
	s.T.Helper()
	if !fileExist(ctx, s, fileName) {
		s.T.Fatalf("expected file %s, but it does not", fileName)
	}
}

func ValidateFileDoesNotExist(ctx context.Context, s *Scenario, fileName string) {
	s.T.Helper()
	if fileExist(ctx, s, fileName) {
		s.T.Fatalf("expected file %s to no exist, but it does", fileName)
	}
}

func ValidateFileIsRegularFile(ctx context.Context, s *Scenario, fileName string) {
	s.T.Helper()

	steps := []string{
		"set -ex",
		fmt.Sprintf("stat --printf=%%F %s | grep 'regular file'", fileName),
	}

	if execScriptOnVMForScenario(ctx, s, strings.Join(steps, "\n")).exitCode != "0" {
		s.T.Fatalf("expected %s to be a regular file, but it is not", fileName)
	}
}

func fileExist(ctx context.Context, s *Scenario, fileName string) bool {
	s.T.Helper()
	if s.IsWindows() {
		steps := []string{
			"$ErrorActionPreference = \"Stop\"",
			fmt.Sprintf("if (Test-Path -Path '%s') { exit 0 } else { exit 1 }", fileName),
		}
		execResult := execScriptOnVMForScenario(ctx, s, strings.Join(steps, "\n"))
		s.T.Logf("stdout: %s\nstderr: %s", execResult.stdout.String(), execResult.stderr.String())
		return execResult.exitCode == "0"
	} else {
		steps := []string{
			"set -ex",
			fmt.Sprintf("test -f %s", fileName),
		}
		execResult := execScriptOnVMForScenario(ctx, s, strings.Join(steps, "\n"))
		return execResult.exitCode == "0"
	}
}

func fileHasContent(ctx context.Context, s *Scenario, fileName string, contents string) bool {
	s.T.Helper()
	require.NotEmpty(s.T, contents, "Test setup failure: Can't validate that a file has contents with an empty string. Filename: %s", fileName)
	if s.IsWindows() {
		steps := []string{
			"$ErrorActionPreference = \"Stop\"",
			fmt.Sprintf("Get-Content %s", fileName),
			fmt.Sprintf("if ( -not ( Test-Path -Path %s ) ) { exit 2 }", fileName),
			fmt.Sprintf("if (Select-String -Path %s -Pattern \"%s\" -SimpleMatch -Quiet) { exit 0 } else { exit 1 }", fileName, contents),
		}
		execResult := execScriptOnVMForScenario(ctx, s, strings.Join(steps, "\n"))
		return execResult.exitCode == "0"
	} else {
		steps := []string{
			"set -ex",
			fmt.Sprintf("sudo cat %s", fileName),
			fmt.Sprintf("(sudo cat %s | grep -q -F -e %q)", fileName, contents),
		}
		execResult := execScriptOnVMForScenario(ctx, s, strings.Join(steps, "\n"))
		return execResult.exitCode == "0"
	}
}

func ValidateFileHasContent(ctx context.Context, s *Scenario, fileName string, contents string) {
	s.T.Helper()
	if !fileHasContent(ctx, s, fileName, contents) {
		s.T.Fatalf("expected file %s to have contents %q, but it does not", fileName, contents)
	}
}

func ValidateFileExcludesContent(ctx context.Context, s *Scenario, fileName string, contents string) {
	s.T.Helper()
	if fileHasContent(ctx, s, fileName, contents) {
		s.T.Fatalf("expected file %s to not have contents %q, but it does", fileName, contents)
	}
}

func ServiceCanRestartValidator(ctx context.Context, s *Scenario, serviceName string, restartTimeoutInSeconds int) {
	s.T.Helper()
	steps := []string{
		"set -ex",
		// Verify the service is active - print the state then verify so we have logs
		fmt.Sprintf("(systemctl -n 5 status %s || true)", serviceName),
		fmt.Sprintf("systemctl is-active %s", serviceName),

		// get the PID of the service, so we can check it's changed
		fmt.Sprintf("INITIAL_PID=`sudo pgrep %s`", serviceName),
		"echo INITIAL_PID: $INITIAL_PID",

		// we use systemctl kill rather than kill -9 because container restrictions stop us sending a kill sig to a process
		fmt.Sprintf("sudo systemctl kill %s", serviceName),

		// sleep for restartTimeoutInSeconds seconds to give the service time tor restart
		fmt.Sprintf("sleep %d", restartTimeoutInSeconds),

		// print the status of the service and then verify it is active.
		fmt.Sprintf("(systemctl -n 5 status %s || true)", serviceName),
		fmt.Sprintf("systemctl is-active %s", serviceName),

		// get the PID of the service after restart, so we can check it's changed
		fmt.Sprintf("POST_PID=`sudo pgrep %s`", serviceName),
		"echo POST_PID: $POST_PID",

		// verify the PID has changed.
		"if [[ \"$INITIAL_PID\" == \"$POST_PID\" ]]; then echo PID did not change after restart, failing validator. ; exit 1; fi",
	}

	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "command to restart service failed")
}

func ValidateSystemdUnitIsRunning(ctx context.Context, s *Scenario, serviceName string) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Print the service status for logging purposes
		fmt.Sprintf("systemctl -n 5 status %s || true", serviceName),
		// Verify the service is active
		fmt.Sprintf("systemctl is-active %s", serviceName),
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0,
		fmt.Sprintf("service %s is not running", serviceName))
}

func ValidateSystemdUnitIsNotRunning(ctx context.Context, s *Scenario, serviceName string) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Print the service status for logging purposes (allow failure)
		fmt.Sprintf("systemctl -n 5 status %s || true", serviceName),
		// Check if service is active - we expect this to fail
		fmt.Sprintf("! systemctl is-active %s", serviceName),
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0,
		fmt.Sprintf("service %s is unexpectedly running", serviceName))
}

func ValidateWindowsServiceIsRunning(ctx context.Context, s *Scenario, serviceName string) {
	s.T.Helper()
	command := []string{
		"$ErrorActionPreference = \"Stop\"",
		// Print the service status for logging purposes
		fmt.Sprintf("Get-Service -Name %s", serviceName),
		// Verify the service is running
		fmt.Sprintf("$service = Get-Service -Name %s", serviceName),
		"if ($service.Status -ne 'Running') { throw \"Service is not running: $($service.Status)\" }",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0,
		fmt.Sprintf("Windows service %s is not running", serviceName))
}

func ValidateWindowsServiceIsNotRunning(ctx context.Context, s *Scenario, serviceName string) {
	s.T.Helper()
	command := []string{
		"$ErrorActionPreference = \"Continue\"",
		// Print the service status for logging purposes
		fmt.Sprintf("Get-Service -Name %s -ErrorAction SilentlyContinue", serviceName),
		// Check if service exists and is not running
		fmt.Sprintf("$service = Get-Service -Name %s -ErrorAction SilentlyContinue", serviceName),
		"if ($service -and $service.Status -eq 'Running') { throw \"Service is unexpectedly running: $($service.Status)\" }",
		"if ($service -and $service.Status -ne 'Running') { Write-Host \"Service exists but is not running: $($service.Status)\" }",
		"if (-not $service) { Write-Host \"Service does not exist\" }",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0,
		fmt.Sprintf("Windows service %s validation failed", serviceName))
}

func ValidateSystemdUnitIsNotFailed(ctx context.Context, s *Scenario, serviceName string) {
	s.T.Helper()
	command := []string{
		"set -ex",
		fmt.Sprintf("systemctl --no-pager -n 5 status %s || true", serviceName),
		fmt.Sprintf("systemctl is-failed %s", serviceName),
	}
	require.NotEqual(
		s.T,
		"0",
		execScriptOnVMForScenario(ctx, s, strings.Join(command, "\n")).exitCode,
		`expected "systemctl is-failed" to exit with a non-zero exit code for unit %q, unit is in a failed state`,
		serviceName,
	)
}

func ValidateUlimitSettings(ctx context.Context, s *Scenario, ulimits map[string]string) {
	s.T.Helper()
	ulimitKeys := make([]string, 0, len(ulimits))
	for k := range ulimits {
		ulimitKeys = append(ulimitKeys, k)
	}

	command := fmt.Sprintf("sudo systemctl cat containerd.service | grep -E -i '%s'", strings.Join(ulimitKeys, "|"))
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 0, "could not read containerd.service file")

	for name, value := range ulimits {
		require.Contains(s.T, execResult.stdout.String(), fmt.Sprintf("%s=%v", name, value), "expected to find %s set to %v, but was not", name, value)
	}
}

func execOnVMForScenarioOnUnprivilegedPod(ctx context.Context, s *Scenario, cmd string) *podExecResult {
	s.T.Helper()
	nonHostPod, err := s.Runtime.Cluster.Kube.GetPodNetworkDebugPodForNode(ctx, s.Runtime.VM.KubeName)
	require.NoError(s.T, err, "failed to get non host debug pod name")
	execResult, err := execOnUnprivilegedPod(ctx, s.Runtime.Cluster.Kube, nonHostPod.Namespace, nonHostPod.Name, cmd)
	require.NoErrorf(s.T, err, "failed to execute command on pod: %v", cmd)
	return execResult
}

func execScriptOnVMForScenario(ctx context.Context, s *Scenario, cmd string) *podExecResult {
	s.T.Helper()
	result, err := execScriptOnVm(ctx, s, s.Runtime.VM.PrivateIP, s.Runtime.Cluster.DebugPod.Name, cmd)
	require.NoError(s.T, err, "failed to execute command on VM")
	return result
}

func execScriptOnVMForScenarioValidateExitCode(ctx context.Context, s *Scenario, cmd string, expectedExitCode int, additionalErrorMessage string) *podExecResult {
	s.T.Helper()
	execResult := execScriptOnVMForScenario(ctx, s, cmd)

	expectedExitCodeStr := fmt.Sprint(expectedExitCode)
	if expectedExitCodeStr != execResult.exitCode {
		s.T.Logf("Command: %s\nStdout: %s\nStderr: %s", cmd, execResult.stdout.String(), execResult.stderr.String())
		s.T.Fatalf("expected exit code %s, but got %s\nCommand: %s\n%s", expectedExitCodeStr, execResult.exitCode, cmd, additionalErrorMessage)
	}
	return execResult
}

func ValidateInstalledPackageVersion(ctx context.Context, s *Scenario, component, version string) {
	s.T.Helper()
	installedCommand := func() string {
		switch s.VHD.OS {
		case config.OSUbuntu:
			return "sudo apt list --installed"
		case config.OSMariner, config.OSAzureLinux:
			return "sudo dnf list installed"
		default:
			s.T.Fatalf("command to get package list isn't implemented for OS %s", s.VHD.OS)
			return ""
		}
	}()
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, installedCommand, 0, "could not get package list")
	for _, line := range strings.Split(execResult.stdout.String(), "\n") {
		if strings.Contains(line, component) && strings.Contains(line, version) {
			s.T.Logf("found %s %s in the installed packages", component, version)
			return
		}
	}
	s.T.Errorf("expected to find %s %s in the installed packages, but did not", component, version)
}

func ValidateKubeletNodeIP(ctx context.Context, s *Scenario) {
	s.T.Helper()
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo cat /etc/default/kubelet", 0, "could not read kubelet config")
	stdout := execResult.stdout.String()

	// Search for "--node-ip" flag and its value.
	matches := regexp.MustCompile(`--node-ip=([a-zA-Z0-9.,]*)`).FindStringSubmatch(stdout)
	require.NotNil(s.T, matches, "could not find kubelet flag --node-ip\nStdout: \n%s", stdout)
	require.GreaterOrEqual(s.T, len(matches), 2, "could not find kubelet flag --node-ip.\nStdout: \n%s", stdout)

	ipAddresses := strings.Split(matches[1], ",") // Could be multiple for dual-stack.
	require.GreaterOrEqual(s.T, len(ipAddresses), 1, "expected at least one --node-ip address, but got none\nStdout: \n%s", stdout)
	require.LessOrEqual(s.T, len(ipAddresses), 2, "expected at most two --node-ip addresses, but got %d\nStdout: \n%s", len(ipAddresses), stdout)

	// Check that each IP is a valid address.
	for _, ipAddress := range ipAddresses {
		require.NotNil(s.T, net.ParseIP(ipAddress), "--node-ip value %q is not a valid IP address\nStdout: \n%s", ipAddress, stdout)
	}
}

func ValidateIMDSRestrictionRule(ctx context.Context, s *Scenario, table string) {
	s.T.Helper()
	cmd := fmt.Sprintf("sudo iptables -t %s -S | grep -q 'AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature'", table)
	execScriptOnVMForScenarioValidateExitCode(ctx, s, cmd, 0, "expected to find IMDS restriction rule, but did not")
}

func ValidateMultipleKubeProxyVersionsExist(ctx context.Context, s *Scenario) {
	s.T.Helper()
	execResult := execScriptOnVMForScenario(ctx, s, "sudo ctr --namespace k8s.io images list | grep kube-proxy | awk '{print $1}' | grep -oE '[0-9]+\\.[0-9]+\\.[0-9]+'")
	if execResult.exitCode != "0" {
		s.T.Errorf("Failed to list kube-proxy images: %s", execResult.stderr)
		return
	}

	versions := bytes.NewBufferString(strings.TrimSpace(execResult.stdout.String()))
	versionMap := make(map[string]struct{})
	for _, version := range strings.Split(versions.String(), "\n") {
		if version != "" {
			versionMap[version] = struct{}{}
		}
	}

	switch len(versionMap) {
	case 0:
		s.T.Errorf("No kube-proxy versions found.")
	case 1:
		s.T.Errorf("Only one kube-proxy version exists: %v", versionMap)
	default:
		s.T.Logf("Multiple kube-proxy versions exist: %v", versionMap)
	}
}

func ValidateKubeletHasNotStopped(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := "sudo journalctl -u kubelet"
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 0, "could not retrieve kubelet logs with journalctl")
	stdout := strings.ToLower(execResult.stdout.String())
	assert.NotContains(s.T, stdout, "stopped kubelet")
	assert.Contains(s.T, stdout, "started kubelet")
}

func ValidateServicesDoNotRestartKubelet(ctx context.Context, s *Scenario) {
	s.T.Helper()
	// grep all filesin /etc/systemd/system/ for /restart\s+kubelet/ and count results
	command := "sudo grep -rl 'restart[[:space:]]\\+kubelet' /etc/systemd/system/"
	execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 1, "expected to find no services containing 'restart kubelet' in /etc/systemd/system/")
}

// ValidateKubeletHasFlags checks kubelet is started with the right flags and configs.
func ValidateKubeletHasFlags(ctx context.Context, s *Scenario, filePath string) {
	s.T.Helper()
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo journalctl -u kubelet", 0, "could not retrieve kubelet logs with journalctl")
	configFileFlags := fmt.Sprintf("FLAG: --config=\"%s\"", filePath)
	require.Containsf(s.T, execResult.stdout.String(), configFileFlags, "expected to find flag %s, but not found", "config")
}

// Waits until the specified resource is available on the given node.
// Returns an error if the resource is not available within the specified timeout period.
func waitUntilResourceAvailable(ctx context.Context, s *Scenario, resourceName string) {
	s.T.Helper()
	nodeName := s.Runtime.VM.KubeName
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.T.Fatalf("context cancelled: %v", ctx.Err())
		case <-ticker.C:
			node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			require.NoError(s.T, err, "failed to get node %q", nodeName)

			if isResourceAvailable(node, resourceName) {
				s.T.Logf("resource %q is available", resourceName)
				return
			}
		}
	}
}

// Checks if the specified resource is available on the node.
func isResourceAvailable(node *corev1.Node, resourceName string) bool {
	for rn, quantity := range node.Status.Allocatable {
		if rn == corev1.ResourceName(resourceName) && quantity.Cmp(resource.MustParse("1")) >= 0 {
			return true
		}
	}
	return false
}

func ValidateContainerd2Properties(ctx context.Context, s *Scenario, versions []string) {
	s.T.Helper()
	require.Lenf(s.T, versions, 1, "Expected exactly one version for moby-containerd but got %d", len(versions))
	// assert versions[0] value starts with '2.'
	require.Truef(s.T, strings.HasPrefix(versions[0], "2."), "expected moby-containerd version to start with '2.', got %v", versions[0])

	ValidateInstalledPackageVersion(ctx, s, "moby-containerd", versions[0])

	execResult := execOnVMForScenarioOnUnprivilegedPod(ctx, s, "containerd config dump ")
	// validate containerd config dump has no warnings
	require.NotContains(s.T, execResult.stdout.String(), "level=warning", "do not expect warning message when converting config file %", execResult.stdout.String())
}

func ValidateContainerRuntimePlugins(ctx context.Context, s *Scenario) {
	// nri plugin is enabled by default
	ValidateDirectoryContent(ctx, s, "/var/run/nri", []string{"nri.sock"})
}

func ValidateNPDGPUCountPlugin(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Check NPD GPU count plugin config exists
		"test -f /etc/node-problem-detector.d/custom-plugin-monitor/gpu_checks/custom-plugin-gpu-count.json",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "NPD GPU count plugin configuration does not exist")
}

func validateNPDCondition(ctx context.Context, s *Scenario, conditionType, conditionReason string, conditionStatus corev1.ConditionStatus, conditionMessage, conditionMessageErr string) {
	s.T.Helper()
	// Wait for NPD to report initial condition
	var condition *corev1.NodeCondition
	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
		if err != nil {
			s.T.Logf("Failed to get node %q: %v", s.Runtime.VM.KubeName, err)
			return false, nil // Continue polling on transient errors
		}

		// Check for condition with correct reason
		for i := range node.Status.Conditions {
			if string(node.Status.Conditions[i].Type) == conditionType && string(node.Status.Conditions[i].Reason) == conditionReason {
				condition = &node.Status.Conditions[i] // Found the partial condition we are looking for
			}

			if strings.Contains(node.Status.Conditions[i].Message, conditionMessage) {
				condition = &node.Status.Conditions[i]
				return true, nil // Found the exact condition we are looking for
			}
		}

		return false, nil // Continue polling until the condition is found or timeout occurs
	})
	if err != nil && condition == nil {
		require.NoError(s.T, err, "timed out waiting for %s condition with reason %s to appear on node %q", conditionType, conditionReason, s.Runtime.VM.KubeName)
	}

	require.NotNil(s.T, condition, "expected to find %s condition with %s reason on node", conditionType, conditionReason)
	require.Equal(s.T, condition.Status, conditionStatus, "expected %s condition to be %s", conditionType, conditionStatus)
	require.Contains(s.T, condition.Message, conditionMessage, conditionMessageErr)
}

func ValidateNPDGPUCountCondition(ctx context.Context, s *Scenario) {
	s.T.Helper()
	// Validate that NPD is reporting healthy GPU count
	validateNPDCondition(ctx, s, "GPUMissing", "NoGPUMissing", corev1.ConditionFalse,
		"All GPUs are present", "expected GPUMissing message to indicate correct count")
}

func ValidateNPDGPUCountAfterFailure(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Stop all services that are holding on to the GPUs
		"sudo systemctl stop nvidia-persistenced.service || true",
		"sudo systemctl stop nvidia-fabricmanager || true",
		// Disable and reset the first GPU
		"sudo nvidia-smi -i 0 -pm 0", // Disable persistence mode
		"sudo nvidia-smi -i 0 -c 0",  // Set compute mode to default
		// sed converts the output into the format needed for NVreg_ExcludeDevices
		"PCI_ID=$(sudo nvidia-smi -i 0 --query-gpu=pci.bus_id --format=csv,noheader | sed 's/^0000//')",
		"echo ${PCI_ID} | tee /tmp/npd_test_disabled_pci_id",
		"echo ${PCI_ID} | sudo tee /sys/bus/pci/drivers/nvidia/unbind", // Reset the GPU
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to disable GPU")

	// Validate that NPD reports the GPU count mismatch
	validateNPDCondition(ctx, s, "GPUMissing", "GPUMissing", corev1.ConditionTrue,
		"Expected to see 8 GPUs but found 7. FaultCode: NHC2009", "expected GPUMissing message to indicate GPU count mismatch")

	command = []string{
		"set -ex",
		"cat /tmp/npd_test_disabled_pci_id | sudo tee /sys/bus/pci/drivers/nvidia/bind",
		"rm -f /tmp/npd_test_disabled_pci_id", // Clean up the temporary file
		"sudo systemctl start nvidia-persistenced.service || true",
	}
	// Put the VM back to the original state, re-enable the GPU.
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to re-enable GPU")
}

func ValidateNPDIBLinkFlappingCondition(ctx context.Context, s *Scenario) {
	s.T.Helper()
	// Validate that NPD is reporting no IB link flapping
	validateNPDCondition(ctx, s, "IBLinkFlapping", "NoIBLinkFlapping", corev1.ConditionFalse,
		"IB link is stable", "expected IBLinkFlapping message to indicate no flapping")
}

func ValidateNPDIBLinkFlappingAfterFailure(ctx context.Context, s *Scenario) {
	s.T.Helper()

	// Simulate IB link flapping
	command := []string{
		"set -ex",
		"echo \"$(date '+%b %d %H:%M:%S') $(hostname) fake error 0: [12346.123456] ib0: lost carrier\" | sudo tee -a /var/log/syslog",
		"sleep 60",
		"echo \"$(date '+%b %d %H:%M:%S') $(hostname) fake error 1: [12346.123456] ib0: lost carrier\" | sudo tee -a /var/log/syslog",
		"sleep 60",
		"echo \"$(date '+%b %d %H:%M:%S') $(hostname) fake error 2: [12346.123456] ib0: lost carrier\" | sudo tee -a /var/log/syslog",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to simulate IB link flapping")

	// Validate that NPD reports IB link flapping
	expectedMessage := "check_ib_link_flapping: IB link flapping detected, multiple IB link flapping events within 6 hours. FaultCode: NHC2005"
	validateNPDCondition(ctx, s, "IBLinkFlapping", "IBLinkFlapping", corev1.ConditionTrue,
		expectedMessage, "expected IBLinkFlapping message to indicate flapping")
}

func ValidateNPDUnhealthyNvidiaDevicePlugin(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Check NPD unhealthy Nvidia device plugin config exists
		"test -f /etc/node-problem-detector.d/custom-plugin-monitor/gpu_checks/custom-plugin-nvidia-device-plugin.json",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "NPD Nvidia device plugin configuration does not exist")
}

func ValidateNPDUnhealthyNvidiaDevicePluginCondition(ctx context.Context, s *Scenario) {
	s.T.Helper()
	// Validate that NPD is reporting healthy Nvidia device plugin
	validateNPDCondition(ctx, s, "UnhealthyNvidiaDevicePlugin", "HealthyNvidiaDevicePlugin", corev1.ConditionFalse,
		"NVIDIA device plugin is running properly", "expected UnhealthyNvidiaDevicePlugin message to indicate healthy status")
}

func ValidateNPDUnhealthyNvidiaDevicePluginAfterFailure(ctx context.Context, s *Scenario) {
	s.T.Helper()
	// Stop Nvidia device plugin systemd service to simulate failure
	command := []string{
		"set -ex",
		"sudo systemctl stop nvidia-device-plugin.service",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to stop Nvidia device plugin service")

	// Validate that NPD reports unhealthy Nvidia device plugin
	validateNPDCondition(ctx, s, "UnhealthyNvidiaDevicePlugin", "UnhealthyNvidiaDevicePlugin", corev1.ConditionTrue,
		"Systemd service nvidia-device-plugin is not active", "expected UnhealthyNvidiaDevicePlugin message to indicate unhealthy status")

	// Restart Nvidia device plugin systemd service
	command = []string{
		"set -ex",
		"sudo systemctl restart nvidia-device-plugin.service || true",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to restart Nvidia device plugin service")
}

func ValidateNPDUnhealthyNvidiaDCGMServices(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Check NPD unhealthy Nvidia DCGM services config exists
		"test -f /etc/node-problem-detector.d/custom-plugin-monitor/gpu_checks/custom-plugin-nvidia-dcgm-services.json",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "NPD Nvidia DCGM services configuration does not exist")
}

func ValidateNPDUnhealthyNvidiaDCGMServicesCondition(ctx context.Context, s *Scenario) {
	s.T.Helper()
	// Validate that NPD is reporting healthy Nvidia DCGM services
	validateNPDCondition(ctx, s, "UnhealthyNvidiaDCGMServices", "HealthyNvidiaDCGMServices", corev1.ConditionFalse,
		"NVIDIA DCGM services are running properly", "expected UnhealthyNvidiaDCGMServices message to indicate healthy status")
}

func ValidateNPDUnhealthyNvidiaDCGMServicesAfterFailure(ctx context.Context, s *Scenario) {
	s.T.Helper()
	// Stop nvidia-dcgm systemd service to simulate failure
	command := []string{
		"set -ex",
		"sudo systemctl stop nvidia-dcgm.service",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to stop Nvidia DCGM service")

	// Validate that NPD reports unhealthy Nvidia DCGM services
	validateNPDCondition(ctx, s, "UnhealthyNvidiaDCGMServices", "UnhealthyNvidiaDCGMServices", corev1.ConditionTrue,
		"Systemd service(s) nvidia-dcgm are not active", "expected UnhealthyNvidiaDCGMServices message to indicate unhealthy status")

	// Stop the nvidia-dcgm-exporter system service to simulate failure
	command = []string{
		"set -ex",
		"sudo systemctl stop nvidia-dcgm-exporter.service",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to stop Nvidia DCGM Exporter service")

	// Validate that NPD still reports unhealthy Nvidia DCGM services
	validateNPDCondition(ctx, s, "UnhealthyNvidiaDCGMServices", "UnhealthyNvidiaDCGMServices", corev1.ConditionTrue,
		"Systemd service(s) nvidia-dcgm nvidia-dcgm-exporter are not active", "expected UnhealthyNvidiaDCGMServices message to indicate unhealthy status for both services")

	// Restart Nvidia DCGM services
	command = []string{
		"set -ex",
		"sudo systemctl restart nvidia-dcgm.service || true",
		"sudo systemctl restart nvidia-dcgm-exporter.service || true",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to restart Nvidia DCGM services")
}

func ValidateRuncVersion(ctx context.Context, s *Scenario, versions []string) {
	s.T.Helper()
	require.Lenf(s.T, versions, 1, "Expected exactly one version for moby-runc but got %d", len(versions))
	// check if versions[0] is great than or equal to 1.2.0
	// check semantic version
	semver, err := semver.ParseTolerant(versions[0])
	require.NoError(s.T, err, "failed to parse semver from moby-runc version")
	require.GreaterOrEqual(s.T, int(semver.Major), 1, "expected moby-runc major version to be at least 1, got %d", semver.Major)
	require.GreaterOrEqual(s.T, int(semver.Minor), 2, "expected moby-runc minor version to be at least 2, got %d", semver.Minor)
	ValidateInstalledPackageVersion(ctx, s, "moby-runc", versions[0])
}

func ValidateWindowsProcessHasCliArguments(ctx context.Context, s *Scenario, processName string, arguments []string) {
	steps := []string{
		fmt.Sprintf("(Get-CimInstance Win32_Process -Filter \"name='%[1]s'\")[0].CommandLine", processName),
	}

	podExecResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate command has parameters - might mean file does not have params, might mean something went wrong")

	actualArgs := strings.Split(podExecResult.stdout.String(), " ")

	for i := 0; i < len(arguments); i++ {
		expectedArgument := arguments[i]
		require.Contains(s.T, actualArgs, expectedArgument)
	}
}

func ValidateWindowsVersionFromWindowsSettings(ctx context.Context, s *Scenario, windowsVersion string) {
	s.T.Helper()
	steps := []string{
		"(Get-ItemProperty -Path \"HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\" -Name BuildLabEx).BuildLabEx",
	}

	jsonBytes := getWindowsSettingsJson()
	osVersion := gjson.GetBytes(jsonBytes, fmt.Sprintf("WindowsBaseVersions.%s.base_image_version", windowsVersion))
	versionSliced := strings.Split(osVersion.String(), ".")
	osMajorVersion := versionSliced[0]

	podExecResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate command has parameters - might mean file does not have params, might mean something went wrong")
	podExecResultStdout := strings.TrimSpace(podExecResult.stdout.String())

	s.T.Logf("Found windows version in windows_settings: \"%s\": \"%s\" (\"%s\")", windowsVersion, osMajorVersion, osVersion)
	s.T.Logf("Windows version returned from VM \"%s\"", podExecResultStdout)

	require.Contains(s.T, podExecResultStdout, osMajorVersion)
}

func ValidateWindowsProductName(ctx context.Context, s *Scenario, productName string) {
	s.T.Helper()
	steps := []string{
		"(Get-ItemProperty \"HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\").ProductName",
	}

	podExecResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate command has parameters - might mean file does not have params, might mean something went wrong")
	podExecResultStdout := strings.TrimSpace(podExecResult.stdout.String())

	require.Contains(s.T, podExecResultStdout, productName)
}

func ValidateWindowsDisplayVersion(ctx context.Context, s *Scenario, displayVersion string) {
	s.T.Helper()
	steps := []string{
		"(Get-ItemProperty \"HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\").DisplayVersion",
	}

	podExecResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate command has parameters - might mean file does not have params, might mean something went wrong")
	podExecResultStdout := strings.TrimSpace(podExecResult.stdout.String())

	s.T.Logf("Windows display version returned from VM \"%s\". Expected display version \"%s\"", podExecResultStdout, displayVersion)

	require.Contains(s.T, podExecResultStdout, displayVersion)
}

func getWindowsSettingsJson() []byte {
	jsonBytes, _ := os.ReadFile("../vhdbuilder/packer/windows/windows_settings.json")
	return jsonBytes
}

func ValidateCiliumIsRunningWindows(ctx context.Context, s *Scenario) {
	s.T.Helper()
	ValidateJsonFileHasField(ctx, s, "/k/azurecni/netconf/10-azure.conflist", "plugins.ipam.type", "azure-cns")
}

func ValidateCiliumIsNotRunningWindows(ctx context.Context, s *Scenario) {
	s.T.Helper()
	ValidateJsonFileDoesNotHaveField(ctx, s, "/k/azurecni/netconf/10-azure.conflist", "plugins.ipam.type", "azure-cns")
}

func ValidateWindowsCiliumIsRunning(ctx context.Context, s *Scenario) {
	s.T.Helper()

	expectedServices := []string{"ebpfcore", "netebpfext", "neteventebpfext", "xdp", "wtc", "hns"}
	for _, serviceName := range expectedServices {
		ValidateWindowsServiceIsRunning(ctx, s, serviceName)
	}

	expectedDlls := []string{"cncapi.dll", "wcnagent.dll"}
	for _, dllName := range expectedDlls {
		ValidateDllLoadedWindows(ctx, s, dllName)
	}
}

func ValidateWindowsCiliumIsNotRunning(ctx context.Context, s *Scenario) {
	s.T.Helper()

	// some of the services used by windows cilium are dependencies of other services, so they may be running even if cilium is not
	// for example, ebpfcore is used by Guest Proxy Agent (GPA), so it may be running even if cilium is not
	// so, we only check that cilium-specific dlls are not loaded, as that is a stronger indication that cilium is not running
	unexpectedDlls := []string{"cncapi.dll", "wcnagent.dll"}
	for _, dllName := range unexpectedDlls {
		ValidateDllIsNotLoadedWindows(ctx, s, dllName)
	}
}

func ValidateDllLoadedWindows(ctx context.Context, s *Scenario, dllName string) {
	s.T.Helper()
	if !dllLoadedWindows(ctx, s, dllName) {
		s.T.Fatalf("expected DLL %s to be loaded, but it is not", dllName)
	}
}

func ValidateDllIsNotLoadedWindows(ctx context.Context, s *Scenario, dllName string) {
	s.T.Helper()
	if dllLoadedWindows(ctx, s, dllName) {
		s.T.Fatalf("expected DLL %s to not be loaded, but it is", dllName)
	}
}

func dllLoadedWindows(ctx context.Context, s *Scenario, dllName string) bool {
	s.T.Helper()

	steps := []string{
		"$ErrorActionPreference = \"Continue\"",
		fmt.Sprintf("tasklist /m %s", dllName),
	}
	execResult := execScriptOnVMForScenario(ctx, s, strings.Join(steps, "\n"))
	dllLoaded := strings.Contains(execResult.stdout.String(), dllName)

	s.T.Logf("stdout: %s\nstderr: %s", execResult.stdout.String(), execResult.stderr.String())
	return dllLoaded
}

func ValidateJsonFileHasField(ctx context.Context, s *Scenario, fileName string, jsonPath string, expectedValue string) {
	s.T.Helper()
	require.Equal(s.T, GetFieldFromJsonObjectOnNode(ctx, s, fileName, jsonPath), expectedValue)
}

func ValidateJsonFileDoesNotHaveField(ctx context.Context, s *Scenario, fileName string, jsonPath string, valueNotToBe string) {
	s.T.Helper()
	require.NotEqual(s.T, GetFieldFromJsonObjectOnNode(ctx, s, fileName, jsonPath), valueNotToBe)
}

func GetFieldFromJsonObjectOnNode(ctx context.Context, s *Scenario, fileName string, jsonPath string) string {
	steps := []string{
		fmt.Sprintf("Get-Content %[1]s", fileName),
		fmt.Sprintf("$content.%s", jsonPath),
	}

	podExecResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate command has parameters - might mean file does not have params, might mean something went wrong")

	return podExecResult.stdout.String()
}

// ValidateTaints checks if the node has the expected taints that are set in the kubelet config with --register-with-taints flag
func ValidateTaints(ctx context.Context, s *Scenario, expectedTaints string) {
	s.T.Helper()
	node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
	require.NoError(s.T, err, "failed to get node %q", s.Runtime.VM.KubeName)
	actualTaints := ""
	for i, taint := range node.Spec.Taints {
		actualTaints += fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
		// add a comma if it's not the last element
		if i < len(node.Spec.Taints)-1 {
			actualTaints += ","
		}
	}
	require.Equal(s.T, expectedTaints, actualTaints, "expected node %q to have taint %q, but got %q", s.Runtime.VM.KubeName, expectedTaints, actualTaints)
}

// ValidateLocalDNSService checks if the localdns service is in the expected state (enabled or disabled).
func ValidateLocalDNSService(ctx context.Context, s *Scenario, state string) {
	s.T.Helper()
	serviceName := "localdns"

	var script string
	switch state {
	case "enabled":
		script = fmt.Sprintf(`set -euo pipefail
svc=%q
systemctl status -n 5 "$svc" || true
active=$(systemctl is-active "$svc" 2>/dev/null || true)
enabled=$(systemctl is-enabled "$svc" 2>/dev/null || true)
echo "localdns: active=$active enabled=$enabled"
test "$active" = "active"   || { echo "expected active, got $active"; exit 1; }
test "$enabled" = "enabled" || { echo "expected enabled, got $enabled"; exit 1; }
`, serviceName)

		execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "localdns should be running and enabled")

	case "disabled":
		script = fmt.Sprintf(`set -euo pipefail
svc=%q
systemctl status -n 5 "$svc" || true
active=$(systemctl is-active "$svc" 2>/dev/null || true)
enabled=$(systemctl is-enabled "$svc" 2>/dev/null || true)
echo "localdns: active=$active enabled=$enabled"
test "$active" = "inactive" || { echo "expected inactive, got $active"; exit 1; }
test "$enabled" = "disabled" || { echo "expected disabled, got $enabled"; exit 1; }
`, serviceName)

		execScriptOnVMForScenarioValidateExitCode(ctx, s, script, 0, "localdns should be stopped and disabled")

	default:
		s.T.Fatalf("unknown state %q; expected 'enable' or 'disable'", state)
	}
}

// ValidateLocalDNSResolution checks if the DNS resolution for an external domain is successful from localdns clusterlistenerIP.
// It uses the 'dig' command to check the DNS resolution and expects a successful response.
func ValidateLocalDNSResolution(ctx context.Context, s *Scenario, server string) {
	s.T.Helper()
	testdomain := "bing.com"
	command := fmt.Sprintf("dig %s +timeout=1 +tries=1", testdomain)
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 0, "dns resolution failed")
	assert.Contains(s.T, execResult.stdout.String(), "status: NOERROR")
	assert.Contains(s.T, execResult.stdout.String(), fmt.Sprintf("SERVER: %s", server))
}

// ValidateJournalctlOutput checks if specific content exists in the systemd service logs
func ValidateJournalctlOutput(ctx context.Context, s *Scenario, serviceName string, expectedContent string) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Get the service logs and check for the expected content
		fmt.Sprintf("sudo journalctl -u %s | grep -q '%s'", serviceName, expectedContent),
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0,
		fmt.Sprintf("expected content '%s' not found in %s service logs", expectedContent, serviceName))
}

func ValidateNodeProblemDetector(ctx context.Context, s *Scenario) {
	command := []string{
		"set -ex",
		// Verify node-problem-detector service is running
		"systemctl is-active node-problem-detector",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "Node Problem Detector (NPD) service validation failed")
}

func ValidateNPDFilesystemCorruption(ctx context.Context, s *Scenario) {
	command := []string{
		"set -ex",
		// Check if the filesystem corruption monitor NPD plugin configuration file exists
		"test -f /etc/node-problem-detector.d/custom-plugin-monitor/custom-fs-corruption-monitor.json",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "NPD Custom Plugin configuration for FilesystemCorruptionProblem not found")

	command = []string{
		"set -ex",
		// Simulate a filesystem corruption problem
		"sudo systemd-run --unit=docker --no-block bash -c 'echo \"structure needs cleaning\"'",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "Failed to simulate filesystem corruption problem")

	// Wait for NPD to detect the problem using Kubernetes native waiting
	var filesystemCorruptionProblem *corev1.NodeCondition
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 6*time.Minute, true, func(ctx context.Context) (bool, error) {
		node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.VM.KubeName, metav1.GetOptions{})
		if err != nil {
			s.T.Logf("Failed to get node %q: %v", s.Runtime.VM.KubeName, err)
			return false, nil // Continue polling on transient errors
		}

		for i := range node.Status.Conditions {
			if node.Status.Conditions[i].Type == "FilesystemCorruptionProblem" && node.Status.Conditions[i].Reason == "FilesystemCorruptionDetected" {
				filesystemCorruptionProblem = &node.Status.Conditions[i]
				return true, nil
			}
		}
		return false, nil // Continue polling
	})
	require.NoError(s.T, err, "timed out waiting for FilesystemCorruptionProblem condition to appear on node %q", s.Runtime.VM.KubeName)

	require.NotNil(s.T, filesystemCorruptionProblem, "expected FilesystemCorruptionProblem condition to be present on node")
	require.Equal(s.T, corev1.ConditionTrue, filesystemCorruptionProblem.Status, "expected FilesystemCorruptionProblem condition to be True on node")
	require.Contains(s.T, filesystemCorruptionProblem.Message, "Found 'structure needs cleaning' in Docker journal.", "expected FilesystemCorruptionProblem condition message to contain: Found 'structure needs cleaning' in Docker journal.")
}

func ValidateEnableNvidiaResource(ctx context.Context, s *Scenario) {
	s.T.Logf("waiting for Nvidia GPU resource to be available")
	waitUntilResourceAvailable(ctx, s, "nvidia.com/gpu")
}

func ValidateNvidiaDevicePluginServiceRunning(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("validating that NVIDIA device plugin systemd service is running")

	command := []string{
		"set -ex",
		"systemctl is-active nvidia-device-plugin.service",
		"systemctl is-enabled nvidia-device-plugin.service",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "NVIDIA device plugin systemd service should be active and enabled")
}

func ValidateNodeAdvertisesGPUResources(ctx context.Context, s *Scenario, gpuCountExpected int64) {
	s.T.Helper()
	s.T.Logf("validating that node advertises GPU resources")
	resourceName := "nvidia.com/gpu"

	// First, wait for the nvidia.com/gpu resource to be available
	waitUntilResourceAvailable(ctx, s, resourceName)

	// Get the node using the Kubernetes client from the test framework
	nodeName := s.Runtime.VM.KubeName
	node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	require.NoError(s.T, err, "failed to get node %q", nodeName)

	// Check if the node advertises GPU capacity
	gpuCapacity, exists := node.Status.Capacity[corev1.ResourceName(resourceName)]
	require.True(s.T, exists, "node should advertise resource %s", resourceName)

	gpuCount := gpuCapacity.Value()
	require.Equal(s.T, gpuCount, gpuCountExpected, "node should advertise %s=%d, but got %s=%d", resourceName, gpuCountExpected, resourceName, gpuCount)
	s.T.Logf("node %s advertises %s=%d resources", nodeName, resourceName, gpuCount)
}

func ValidateGPUWorkloadSchedulable(ctx context.Context, s *Scenario, gpuCount int) {
	s.T.Helper()
	s.T.Logf("validating that GPU workloads can be scheduled")

	// Wait for resources to be available and add delay for device health
	waitUntilResourceAvailable(ctx, s, "nvidia.com/gpu")
	time.Sleep(20 * time.Second) // Same delay as existing GPU tests

	// Create a GPU test pod using the same pattern as podRunNvidiaWorkload
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-gpu-test", s.Runtime.VM.KubeName),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "gpu-test-container",
					Image: "mcr.microsoft.com/azuredocs/samples-tf-mnist-demo:gpu",
					Args: []string{
						"--max-steps", "1",
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"nvidia.com/gpu": resource.MustParse(fmt.Sprintf("%d", gpuCount)),
						},
					},
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": s.Runtime.VM.KubeName,
			},
		},
	}

	ValidatePodRunning(ctx, s, pod)

	s.T.Logf("GPU workload is schedulable and runs successfully")
}

// ValidatePubkeySSHDisabled validates that SSH with private key authentication is disabled by checking sshd_config
func ValidatePubkeySSHDisabled(ctx context.Context, s *Scenario) {
	s.T.Helper()

	// Part 1. Use VMSS RunCommand to check sshd_config directly on the node
	resp, err := RunCommand(ctx, s, `#!/bin/bash
# Check if PubkeyAuthentication is disabled in sshd_config
if grep -q "^PubkeyAuthentication no" /etc/ssh/sshd_config; then
    echo "SUCCESS: PubkeyAuthentication is disabled"
    exit 0
else
    echo "FAILED: PubkeyAuthentication is not properly disabled"
    echo "Current sshd_config content related to PubkeyAuthentication:"
    grep -i "PubkeyAuthentication" /etc/ssh/sshd_config || echo "No PubkeyAuthentication setting found"
    exit 1
fi`)
	require.NoError(s.T, err, "Failed to run command to check sshd_config")
	respJson, err := resp.MarshalJSON()
	require.NoError(s.T, err, "Failed to marshal response")
	s.T.Logf("Run command output: %s", string(respJson))

	// Parse the JSON response to extract the output and exit code
	respString := string(respJson)

	// Check if the command execution was successful by looking for our success message in the output
	if !strings.Contains(respString, "SUCCESS: PubkeyAuthentication is disabled") {
		s.T.Fatalf("PubkeyAuthentication is not properly disabled. Full response: %s", respString)
	}

	// Part 2. Check cannot SSH with private key (expect failure)
	err = validateSSHConnectivity(ctx, s)
	require.Error(s.T, err, "Expected SSH connection with private key to fail, but it succeeded")
	if !strings.Contains(err.Error(), "Permission denied") {
		s.T.Fatalf("Expected permission denied error, but got: %v", err)
	}

	s.T.Logf("PubkeyAuthentication is properly disabled as expected")
}

// ValidateSSHServiceDisabled validates that the SSH daemon service is disabled and stopped on the node
func ValidateSSHServiceDisabled(ctx context.Context, s *Scenario) {
	s.T.Helper()

	// Use VMSS RunCommand to check SSH service status directly on the node
	// Ubuntu uses 'ssh' as service name, while AzureLinux and Mariner use 'sshd'
	runPoller, err := config.Azure.VMSSVM.BeginRunCommand(ctx, *s.Runtime.Cluster.Model.Properties.NodeResourceGroup, s.Runtime.VMSSName, *s.Runtime.VM.VM.InstanceID, armcompute.RunCommandInput{
		CommandID: to.Ptr("RunShellScript"),
		Script: []*string{to.Ptr(`#!/bin/bash
# Determine the correct SSH service name based on the distro
# Ubuntu uses 'ssh', AzureLinux and Mariner use 'sshd'
if [ -f /etc/os-release ]; then
    . /etc/os-release
    if [[ "$ID" == "ubuntu" ]]; then
        SSH_SERVICE="ssh"
    else
        SSH_SERVICE="sshd"
    fi
else
    # Default to sshd if we can't determine the OS
    SSH_SERVICE="sshd"
fi

echo "Detected SSH service name: $SSH_SERVICE"

# Check SSH service status
status_output=$(systemctl status "$SSH_SERVICE" 2>&1)
echo "SSH service status output:"
echo "$status_output"

# Check if the service is inactive (dead) and disabled
if echo "$status_output" | grep -q "Active: inactive (dead)"; then
    if echo "$status_output" | grep -q "Loaded:.*disabled"; then
        echo "SUCCESS: SSH service is disabled and stopped"
        exit 0
    else
        echo "FAILED: SSH service is inactive but not disabled"
        exit 1
    fi
else
    echo "FAILED: SSH service is not inactive"
    exit 1
fi`)},
	}, nil)
	require.NoError(s.T, err, "Failed to run command to check SSH service status")

	runResp, err := runPoller.PollUntilDone(ctx, nil)
	require.NoError(s.T, err, "Failed to complete command to check SSH service status")

	// Parse the response to check the result
	respJson, err := runResp.MarshalJSON()
	require.NoError(s.T, err, "Failed to marshal run command response")
	s.T.Logf("Run command output: %s", string(respJson))

	// Parse the JSON response to extract the output
	respString := string(respJson)

	// Check if the command execution was successful by looking for our success message in the output
	if !strings.Contains(respString, "SUCCESS: SSH service is disabled and stopped") {
		s.T.Fatalf("SSH service is not properly disabled and stopped. Full response: %s", respString)
	}

	s.T.Logf("SSH service is properly disabled and stopped as expected")
}

func ValidateNvidiaDCGMExporterSystemDServiceRunning(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Verify nvidia-dcgm service is running
		"systemctl is-active nvidia-dcgm",
		// Verify nvidia-dcgm-exporter service is running
		"systemctl is-active nvidia-dcgm-exporter",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "Nvidia DCGM Exporter service validation failed")
}

func ValidateNvidiaDCGMExporterIsScrapable(ctx context.Context, s *Scenario) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Check if nvidia-dcgm-exporter is scrapable on port 19400
		"curl -f http://localhost:19400/metrics",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "Nvidia DCGM Exporter is not scrapable on port 19400")
}

func ValidateNvidiaDCGMExporterScrapeCommonMetric(ctx context.Context, s *Scenario, metric string) {
	s.T.Helper()
	command := []string{
		"set -ex",
		// Verify the most universal GPU metric is present
		"curl -s http://localhost:19400/metrics | grep -q '" + metric + "'",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "Nvidia DCGM Exporter is not returning "+metric)
}

func ValidateMIGModeEnabled(ctx context.Context, s *Scenario) {
	s.T.Helper()
	s.T.Logf("validating that MIG mode is enabled")

	command := []string{
		"set -ex",
		// Grep to verify it contains 'Enabled' - this will fail if MIG is disabled
		"sudo nvidia-smi --query-gpu=mig.mode.current --format=csv,noheader | grep -i 'Enabled'",
	}
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "MIG mode is not enabled")

	stdout := strings.TrimSpace(execResult.stdout.String())
	s.T.Logf("MIG mode status: %s", stdout)
	require.Contains(s.T, stdout, "Enabled", "expected MIG mode to be enabled, but got: %s", stdout)
	s.T.Logf("MIG mode is enabled")
}

func ValidateMIGInstancesCreated(ctx context.Context, s *Scenario, migProfile string) {
	s.T.Helper()
	s.T.Logf("validating that MIG instances are created with profile %s", migProfile)

	command := []string{
		"set -ex",
		// List MIG devices using nvidia-smi
		"sudo nvidia-smi mig -lgi",
		// Ensure the output contains the expected MIG profile (will fail if "No MIG-enabled devices found")
		"sudo nvidia-smi mig -lgi | grep -v 'No MIG-enabled devices found' | grep -q '" + migProfile + "'",
	}
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "MIG instances with profile "+migProfile+" were not found")

	stdout := execResult.stdout.String()
	require.Contains(s.T, stdout, migProfile, "expected to find MIG profile %s in output, but did not.\nOutput:\n%s", migProfile, stdout)
	require.NotContains(s.T, stdout, "No MIG-enabled devices found", "no MIG devices were created.\nOutput:\n%s", stdout)
	s.T.Logf("MIG instances with profile %s are created", migProfile)
}

// ValidateIPTablesCompatibleWithCiliumEBPF validates that all iptables rules in each table match the provided patterns which are accounted for
// when eBPF host routing is enabled.
func ValidateIPTablesCompatibleWithCiliumEBPF(ctx context.Context, s *Scenario) {
	s.T.Helper()
	tablePatterns, globalPatterns := getIPTablesRulesCompatibleWithEBPFHostRouting()
	tables := []string{"filter", "mangle", "nat", "raw", "security"}
	success := true

	for _, table := range tables {
		// Get the rules for this table
		command := fmt.Sprintf("sudo iptables -t %s -S", table)
		execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 0, fmt.Sprintf("failed to get iptables rules for table %s", table))

		stdout := execResult.stdout.String()
		rules := strings.Split(strings.TrimSpace(stdout), "\n")

		// Get patterns for this table
		patterns := tablePatterns[table]
		if patterns == nil {
			patterns = []string{}
		}

		// Combine with global patterns
		allPatterns := append([]string{}, globalPatterns...)
		allPatterns = append(allPatterns, patterns...)

		// Check each rule
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)
			if rule == "" {
				continue
			}

			matched := false
			for _, pattern := range allPatterns {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" {
					continue
				}

				// Try regex match
				matched, _ = regexp.MatchString(pattern, rule)
				if matched {
					break
				}

				// Also try exact match for non-regex patterns
				if strings.Contains(rule, pattern) {
					matched = true
					break
				}
			}

			if !matched {
				s.T.Logf("Rule in table %s did not match any pattern: %s", table, rule)
				success = false
			}
		}
	}

	require.True(
		s.T,
		success,
		"Rules found that do not match any of the given patterns. See previous log lines for details. "+
			"This may indicate an unsupported iptables rule when eBPF host routing is enabled. "+
			"Contact acndp@microsoft.com for details.",
	)
}

// getIPTablesRulesCompatibleWithEBPFHostRouting returns the expected iptables patterns that are accounted for when EBPF host routing is enabled.
// If tests are failing due to unexpected iptables rules, it is because an iptables rule has been found, that was not accounted for in the implementation
// of the eBPF host routing feature in Cilium CNI. In eBPF host routing mode, iptables rules in the host network namespace are bypassed for pod
// traffic. So, any functionality that is built using iptables needs an equivalent non-iptables implementation that works in Cilium's eBPF host routing
// mode. For guidance on how this may be done, please contact acndp@microsoft.com (Azure Container Networking Dataplane team). Once the feature
// is supported in eBPF host routing mode, or is blocked from being enabled alongside eBPF host routing mode, you can update this list.
func getIPTablesRulesCompatibleWithEBPFHostRouting() (map[string][]string, []string) {
	tablePatterns := map[string][]string{
		"filter": {
			`-A FORWARD -d 168.63.129.16/32 -p tcp -m tcp --dport 32526 -j DROP`,
			`-A FORWARD -d 168.63.129.16/32 -p tcp -m tcp --dport 80 -j DROP`,
		},
		"mangle": {
			`-A FORWARD -d 168\.63\.129\.16/32 -p tcp -m tcp --dport 80 -j DROP`,
			`-A FORWARD -d 168\.63\.129\.16/32 -p tcp -m tcp --dport 32526 -j DROP`,
		},
		"nat": {
			`-A POSTROUTING -j SWIFT`,
			`-A SWIFT -s`,
			`-A POSTROUTING -j SWIFT-POSTROUTING`,
			`-A SWIFT-POSTROUTING -s`,
		},
		"raw": {
			`^-A (PREROUTING|OUTPUT) -d 169\.254\.10\.(10|11)\/32 -p (tcp|udp) -m comment --comment "localdns: skip conntrack" -m (tcp|udp) --dport 53 -j NOTRACK$`,
		},
		"security": {
			`-A OUTPUT -d 168\.63\.129\.16/32 -p tcp -m tcp --dport 53 -j ACCEPT`,
			`-A OUTPUT -d 168\.63\.129\.16/32 -p tcp -m owner --uid-owner 0 -j ACCEPT`,
			`-A OUTPUT -d 168\.63\.129\.16/32 -p tcp -m conntrack --ctstate INVALID,NEW -j DROP`,
		},
	}

	globalPatterns := []string{
		`^-N .*`,
		`^-P .*`,
		`^-A (KUBE-SERVICES|KUBE-EXTERNAL-SERVICES|KUBE-NODEPORTS|KUBE-POSTROUTING|KUBE-MARK-MASQ|KUBE-FORWARD|KUBE-PROXY-FIREWALL|KUBE-PROXY-CANARY|KUBE-FIREWALL|KUBE-MARK-DROP) .*`,
		`^-A (KUBE-SEP|KUBE-SVC)`,
		`^-A .* -j (KUBE-SEP|KUBE-SVC|KUBE-SERVICES|KUBE-EXTERNAL-SERVICES|KUBE-NODEPORTS|KUBE-POSTROUTING|KUBE-MARK-MASQ|KUBE-FORWARD|KUBE-PROXY-FIREWALL|KUBE-PROXY-CANARY|KUBE-FIREWALL|KUBE-MARK-DROP)`,
		`^-A IP-MASQ-AGENT`,
		`^-A .* -j IP-MASQ-AGENT`,
		`^.*--comment.*cilium:`,
		`^.*--comment.*cilium-feeder:`,
		`-A FORWARD ! -s (?:\d{1,3}\.){3}\d{1,3}/32 -d 169.254.169.254/32 -p tcp -m tcp --dport 80 -m comment --comment "AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature" -j DROP`,
	}

	return tablePatterns, globalPatterns
}

// ValidateAppArmorBasic validates that AppArmor is running without requiring aa-status
func ValidateAppArmorBasic(ctx context.Context, s *Scenario) {
	s.T.Helper()

	// Check if AppArmor module is enabled in the kernel
	command := []string{
		"set -ex",
		"cat /sys/module/apparmor/parameters/enabled",
	}
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to check AppArmor kernel parameter")
	stdout := strings.TrimSpace(execResult.stdout.String())
	require.Equal(s.T, "Y", stdout, "expected AppArmor to be enabled in kernel")

	// Check if apparmor.service is active
	command = []string{
		"set -ex",
		"systemctl is-active apparmor.service",
	}
	execResult = execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "apparmor.service is not active")
	stdout = strings.TrimSpace(execResult.stdout.String())
	require.Equal(s.T, "active", stdout, "expected apparmor.service to be active")

	// Check if AppArmor is enforcing by checking current process profile
	command = []string{
		"set -ex",
		"cat /proc/self/attr/apparmor/current",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to check AppArmor current profile")
	// Any output indicates AppArmor is active (profile will be shown)
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

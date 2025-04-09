package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ValidateDirectoryContent(ctx context.Context, s *Scenario, path string, files []string) {
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
	command := []string{
		"set -ex",
		"sudo nvidia-smi",
	}
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 1, "")
	stderr := execResult.stderr.String()
	require.Contains(s.T, stderr, "nvidia-smi: command not found", "expected stderr to contain 'nvidia-smi: command not found', but got %q", stderr)
}

func ValidateNvidiaSMIInstalled(ctx context.Context, s *Scenario) {
	command := []string{"set -ex", "sudo nvidia-smi"}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "could not execute nvidia-smi command")
}

func ValidateNvidiaModProbeInstalled(ctx context.Context, s *Scenario) {
	command := []string{
		"set -ex",
		"sudo nvidia-modprobe",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "could not execute nvidia-modprobe command")
}

func ValidateNvidiaGRIDLicenseValid(ctx context.Context, s *Scenario) {
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
	command := []string{
		"set -ex",
		// Check that nvidia-persistenced.service is active by capturing its is-active output
		"active_status=$(sudo systemctl is-active nvidia-persistenced.service)",
		"if [ \"$active_status\" != \"active\" ]; then echo \"nvidia-gridd is not active: $active_status\"; exit 1; fi",
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "failed to validate nvidia-persistenced.service status")
}


func ValidateNonEmptyDirectory(ctx context.Context, s *Scenario, dirName string) {
	command := []string{
		"set -ex",
		fmt.Sprintf("sudo ls -1q %s | grep -q '^.*$' && true || false", dirName),
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(command, "\n"), 0, "either could not find expected file, or something went wrong")
}

func ValidateFileHasContent(ctx context.Context, s *Scenario, fileName string, contents string) {
	if s.VHD.OS == config.OSWindows {
		steps := []string{
			fmt.Sprintf("dir %[1]s", fileName),
			fmt.Sprintf("Get-Content %[1]s", fileName),
			fmt.Sprintf("if (Select-String -Path %s -Pattern \"%s\" -SimpleMatch -Quiet) { return 1 } else { return 0 }", fileName, contents),
		}

		execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate file has contents - might mean file does not have contents, might mean something went wrong")
	} else {
		steps := []string{
			"set -ex",
			fmt.Sprintf("ls -la %[1]s", fileName),
			fmt.Sprintf("sudo cat %[1]s", fileName),
			fmt.Sprintf("(sudo cat %[1]s | grep -q -F -e %[2]q)", fileName, contents),
		}

		execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate file has contents - might mean file does not have contents, might mean something went wrong")
	}
}

func ValidateFileExcludesContent(ctx context.Context, s *Scenario, fileName string, contents string) {
	require.NotEqual(s.T, "", contents, "Test setup failure: Can't validate that a file excludes an empty string. Filename: %s", fileName)

	steps := []string{
		"set -ex",
		fmt.Sprintf("test -f %[1]s || exit 0", fileName),
		fmt.Sprintf("ls -la %[1]s", fileName),
		fmt.Sprintf("sudo cat %[1]s", fileName),
		fmt.Sprintf("(sudo cat %[1]s | grep -q -v -F -e %[2]q)", fileName, contents),
	}
	execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate file excludes contents - might mean file does have contents, might mean something went wrong")
}

func ServiceCanRestartValidator(ctx context.Context, s *Scenario, serviceName string, restartTimeoutInSeconds int) {
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

func ValidateUlimitSettings(ctx context.Context, s *Scenario, ulimits map[string]string) {
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
	nonHostPod, err := s.Runtime.Cluster.Kube.GetPodNetworkDebugPodForNode(ctx, s.Runtime.KubeNodeName, s.T)
	require.NoError(s.T, err, "failed to get non host debug pod name")
	execResult, err := execOnUnprivilegedPod(ctx, s.Runtime.Cluster.Kube, nonHostPod.Namespace, nonHostPod.Name, cmd)
	require.NoErrorf(s.T, err, "failed to execute command on pod: %v", cmd)
	return execResult
}

func execScriptOnVMForScenario(ctx context.Context, s *Scenario, cmd string) *podExecResult {
	script := Script{
		script: cmd,
	}
	if s.VHD.OS == config.OSWindows {
		script.interpreter = Powershell
	} else {
		script.interpreter = Bash
	}

	result, err := execScriptOnVm(ctx, s, s.Runtime.VMPrivateIP, s.Runtime.Cluster.DebugPod.Name, string(s.Runtime.SSHKeyPrivate), script)
	require.NoError(s.T, err, "failed to execute command on VM")
	return result
}

func execScriptOnVMForScenarioValidateExitCode(ctx context.Context, s *Scenario, cmd string, expectedExitCode int, additionalErrorMessage string) *podExecResult {
	execResult := execScriptOnVMForScenario(ctx, s, cmd)

	expectedExitCodeStr := fmt.Sprint(expectedExitCode)
	require.Equal(s.T, expectedExitCodeStr, execResult.exitCode, "exec command failed with exit code %q, expected exit code %s\nCommand: %s\nAdditional detail: %s\nSTDOUT:\n%s\n\nSTDERR:\n%s", execResult.exitCode, expectedExitCodeStr, cmd, additionalErrorMessage, execResult.stdout, execResult.stderr)

	return execResult
}

func ValidateInstalledPackageVersion(ctx context.Context, s *Scenario, component, version string) {
	s.T.Logf("assert %s %s is installed on the VM", component, version)
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
	containsComponent := func() bool {
		for _, line := range strings.Split(execResult.stdout.String(), "\n") {
			if strings.Contains(line, component) && strings.Contains(line, version) {
				return true
			}
		}
		return false
	}()
	if !containsComponent {
		s.T.Logf("expected to find %s %s in the installed packages, but did not", component, version)
		s.T.Fail()
	}
}

func ValidateKubeletNodeIP(ctx context.Context, s *Scenario) {
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
	cmd := fmt.Sprintf("sudo iptables -t %s -S | grep -q 'AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature'", table)
	execScriptOnVMForScenarioValidateExitCode(ctx, s, cmd, 0, "expected to find IMDS restriction rule, but did not")
}

func ValidateMultipleKubeProxyVersionsExist(ctx context.Context, s *Scenario) {
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

func ValidateContainerdWASMShims(ctx context.Context, s *Scenario) {
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo cat /etc/containerd/config.toml", 0, "could not get containerd config content")
	expectedShims := []string{
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin]`,
		`runtime_type = "io.containerd.spin.v2"`,
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight]`,
		`runtime_type = "io.containerd.slight-v0-3-0.v1"`,
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-3-0]`,
		`runtime_type = "io.containerd.spin-v0-3-0.v1"`,
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-3-0]`,
		`runtime_type = "io.containerd.slight-v0-3-0.v1"`,
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-5-1]`,
		`runtime_type = "io.containerd.spin-v0-5-1.v1"`,
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-5-1]`,
		`runtime_type = "io.containerd.slight-v0-5-1.v1"`,
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-8-0]`,
		`runtime_type = "io.containerd.spin-v0-8-0.v1"`,
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.slight-v0-8-0]`,
		`runtime_type = "io.containerd.slight-v0-8-0.v1"`,
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.wws-v0-8-0]`,
		`runtime_type = "io.containerd.wws-v0-8-0.v1"`,
		`[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.spin-v0-15-1]`,
		`runtime_type = "io.containerd.spin.v2"`,
	}
	for i := 0; i < len(expectedShims); i += 2 {
		section := expectedShims[i]
		runtimeType := expectedShims[i+1]
		require.Contains(s.T, execResult.stdout.String(), section, "expected to find section in containerd config.toml, but it was not found")
		require.Contains(s.T, execResult.stdout.String(), runtimeType, "expected to find section in containerd config.toml, but it was not found")
	}
}

func ValidateKubeletHasNotStopped(ctx context.Context, s *Scenario) {
	command := "sudo journalctl -u kubelet"
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 0, "could not retrieve kubelet logs with journalctl")
	assert.NotContains(s.T, execResult.stdout.String(), "Stopped Kubelet")
	assert.Contains(s.T, execResult.stdout.String(), "Started Kubelet")
}

func ValidateServicesDoNotRestartKubelet(ctx context.Context, s *Scenario) {
	// grep all filesin /etc/systemd/system/ for /restart\s+kubelet/ and count results
	command := "sudo grep -rl 'restart[[:space:]]\\+kubelet' /etc/systemd/system/"
	execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 1, "expected to find no services containing 'restart kubelet' in /etc/systemd/system/")
}

// ValidateKubeletHasFlags checks kubelet is started with the right flags and configs.
func ValidateKubeletHasFlags(ctx context.Context, s *Scenario, filePath string) {
	execResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, "sudo journalctl -u kubelet", 0, "could not retrieve kubelet logs with journalctl")
	configFileFlags := fmt.Sprintf("FLAG: --config=\"%s\"", filePath)
	require.Containsf(s.T, execResult.stdout.String(), configFileFlags, "expected to find flag %s, but not found", "config")
}

func ValidatePodUsingNVidiaGPU(ctx context.Context, s *Scenario) {
	s.T.Logf("validating pod using nvidia GPU")
	// NVidia pod can be ready, but resources may not be available yet
	// a hacky way to ensure the next pod is schedulable
	waitUntilResourceAvailable(ctx, s, "nvidia.com/gpu")
	// device can be allocatable, but not healthy
	// ugly hack, but I don't see a better solution
	time.Sleep(20 * time.Second)
	ensurePod(ctx, s, podRunNvidiaWorkload(s))
}

// Waits until the specified resource is available on the given node.
// Returns an error if the resource is not available within the specified timeout period.
func waitUntilResourceAvailable(ctx context.Context, s *Scenario, resourceName string) {
	nodeName := s.Runtime.KubeNodeName
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

func ValidateRunc12Properties(ctx context.Context, s *Scenario, versions []string) {
	require.Lenf(s.T, versions, 1, "Expected exactly one version for moby-runc but got %d", len(versions))
	// assert versions[0] value starts with '1.2.'
	require.Truef(s.T, strings.HasPrefix(versions[0], "1.2."), "expected moby-runc version to start with '1.2.', got %v", versions[0])
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
	steps := []string{
		"(Get-ItemProperty -Path \"HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\" -Name BuildLabEx).BuildLabEx",
	}

	jsonBytes := getWindowsSettingsJson()
	osVersion := gjson.GetBytes(jsonBytes, fmt.Sprintf("WindowsBaseVersions.%s.base_image_version", windowsVersion))
	versionSliced := strings.Split(osVersion.String(), ".")
	osMajorVersion := versionSliced[0]

	podExecResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate command has parameters - might mean file does not have params, might mean something went wrong")
	podExecResultStdout := strings.TrimSpace(podExecResult.stdout.String())

	s.T.Logf("Found windows version in windows_settings: %s: %s (%s)", windowsVersion, osMajorVersion, osVersion)
	s.T.Logf("Windows version returned from VM  %s", podExecResultStdout)

	require.Contains(s.T, podExecResultStdout, osMajorVersion)
}

func ValidateWindowsProductName(ctx context.Context, s *Scenario, productName string) {
	steps := []string{
		"(Get-ItemProperty \"HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\").ProductName",
	}

	podExecResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate command has parameters - might mean file does not have params, might mean something went wrong")
	podExecResultStdout := strings.TrimSpace(podExecResult.stdout.String())

	s.T.Logf("Winddows product name from VM  %s. Expected product name %s", podExecResultStdout, productName)

	require.Contains(s.T, podExecResultStdout, productName)
}

func ValidateWindowsDisplayVersion(ctx context.Context, s *Scenario, displayVersion string) {
	steps := []string{
		"(Get-ItemProperty \"HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\").DisplayVersion",
	}

	podExecResult := execScriptOnVMForScenarioValidateExitCode(ctx, s, strings.Join(steps, "\n"), 0, "could not validate command has parameters - might mean file does not have params, might mean something went wrong")
	podExecResultStdout := strings.TrimSpace(podExecResult.stdout.String())

	s.T.Logf("Winddows display version returned from VM  %s. Expected display version %s", podExecResultStdout, displayVersion)

	require.Contains(s.T, podExecResultStdout, displayVersion)
}

func getWindowsSettingsJson() []byte {
	jsonBytes, _ := os.ReadFile("../vhdbuilder/packer/windows/windows_settings.json")
	return jsonBytes
}

func ValidateCiliumIsRunningWindows(ctx context.Context, s *Scenario) {
	ValidateJsonFileHasField(ctx, s, "/k/azurecni/netconf/10-azure.conflist", "plugins.ipam.type", "azure-cns")
}

func ValidateCiliumIsNotRunningWindows(ctx context.Context, s *Scenario) {
	ValidateJsonFileDoesNotHaveField(ctx, s, "/k/azurecni/netconf/10-azure.conflist", "plugins.ipam.type", "azure-cns")
}

func ValidateJsonFileHasField(ctx context.Context, s *Scenario, fileName string, jsonPath string, expectedValue string) {
	require.Equal(s.T, GetFieldFromJsonObjectOnNode(ctx, s, fileName, jsonPath), expectedValue)
}

func ValidateJsonFileDoesNotHaveField(ctx context.Context, s *Scenario, fileName string, jsonPath string, valueNotToBe string) {
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
	node, err := s.Runtime.Cluster.Kube.Typed.CoreV1().Nodes().Get(ctx, s.Runtime.KubeNodeName, metav1.GetOptions{})
	require.NoError(s.T, err, "failed to get node %q", s.Runtime.KubeNodeName)
	actualTaints := ""
	for i, taint := range node.Spec.Taints {
		actualTaints += fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)
		// add a comma if it's not the last element
		if i < len(node.Spec.Taints)-1 {
			actualTaints += ","
		}
	}
	require.Equal(s.T, expectedTaints, actualTaints, "expected node %q to have taint %q, but got %q", s.Runtime.KubeNodeName, expectedTaints, actualTaints)
}

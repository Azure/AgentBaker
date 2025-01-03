package e2e

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ValidateDirectoryContent(ctx context.Context, s *Scenario, path string, files []string) {
	command := fmt.Sprintf("ls -la %s", path)
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "0", execResult.exitCode, "validator command terminated with exit code %q but expected code 0", execResult.exitCode)
	for _, file := range files {
		require.Contains(s.T, execResult.stdout.String(), file, "expected to find file %s within directory %s, but did not", file, path)
	}
}

func ValidateSysctlConfig(ctx context.Context, s *Scenario, customSysctls map[string]string) {
	keysToCheck := make([]string, len(customSysctls))
	for k := range customSysctls {
		keysToCheck = append(keysToCheck, k)
	}
	execResult := execOnVMForScenario(ctx, s, fmt.Sprintf("sysctl %s | sed -E 's/([0-9])\\s+([0-9])/\\1 \\2/g'", strings.Join(keysToCheck, " ")))
	require.Equal(s.T, "0", execResult.exitCode, "sysctl command terminated with exit code %q but expected code 0", execResult.exitCode)
	for name, value := range customSysctls {
		require.Contains(s.T, execResult.stdout.String(), fmt.Sprintf("%s = %v", name, value), "expected to find %s set to %v, but was not", name, value)
	}
}

func ValidateNvidiaSMINotInstalled(ctx context.Context, s *Scenario) {
	command := "nvidia-smi"
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "1", execResult.exitCode, "expected nvidia-smi not to be installed and return exit code 1, but got %q", execResult.exitCode)
	require.Contains(s.T, execResult.stderr.String(), "nvidia-smi: command not found", "expected stderr to contain 'nvidia-smi: command not found', but got %q", execResult.stderr.String())
}

func ValidateNvidiaSMIInstalled(ctx context.Context, s *Scenario) {
	command := "nvidia-smi"
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "0", execResult.exitCode, "expected nvidia-smi to be installed and return exit code 0, but got %q", execResult.exitCode)
}

func ValidateNvidiaModProbeInstalled(ctx context.Context, s *Scenario) {
	command := "nvidia-modprobe"
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "0", execResult.exitCode, "expected nvidia-modprobe to be installed and return exit code 0, but got %q", execResult.exitCode)
}

func ValidateNonEmptyDirectory(ctx context.Context, s *Scenario, dirName string) {
	command := fmt.Sprintf("ls -1q %s | grep -q '^.*$' && true || false", dirName)
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "0", execResult.exitCode, "expected to find a file in directory %s, but did not", dirName)
}

func ValidateFileHasContent(ctx context.Context, s *Scenario, fileName string, contents string) {
	steps := []string{
		fmt.Sprintf("ls -la %[1]s", fileName),
		fmt.Sprintf("sudo cat %[1]s", fileName),
		fmt.Sprintf("(sudo cat %[1]s | grep -q %[2]q)", fileName, contents),
	}

	command := makeExecutableCommand(steps)
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "0", execResult.exitCode, "expected to find a file '%s' with contents '%s' but did not", fileName, contents)
}

func ValidateFileExcludesContent(ctx context.Context, s *Scenario, fileName string, contents string, contentsName string) {
	command := fmt.Sprintf("grep -q -F '%s' '%s'", contents, fileName)
	execResult := execOnVMForScenario(ctx, s, command)
	require.NotEqual(s.T, "0", execResult.exitCode, "expected to find a file '%s' without %s but did not", fileName, contentsName)
}

// this function is just used to remove some bash specific tokens so we can echo the command to stdout.
func cleanse(str string) string {
	return strings.Replace(str, "'", "", -1)
}

func makeExecutableCommand(steps []string) string {
	stepsWithEchos := make([]string, len(steps)*2)

	for i, s := range steps {
		stepsWithEchos[i*2] = fmt.Sprintf("echo '%s'", cleanse(s))
		stepsWithEchos[i*2+1] = s
	}

	// quote " quotes and $ vars
	joinedCommand := strings.Join(stepsWithEchos, " && ")
	quotedCommand := strings.Replace(joinedCommand, "'", "'\"'\"'", -1)

	command := fmt.Sprintf("bash -c '%s'", quotedCommand)

	return command
}

func ServiceCanRestartValidator(ctx context.Context, s *Scenario, serviceName string, restartTimeoutInSeconds int) {
	steps := []string{
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

	command := makeExecutableCommand(steps)
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "0", execResult.exitCode, "service kill and check terminated with exit code %q (expected 0)", execResult.exitCode)
}

func ValidateUlimitSettings(ctx context.Context, s *Scenario, ulimits map[string]string) {
	ulimitKeys := make([]string, 0, len(ulimits))
	for k := range ulimits {
		ulimitKeys = append(ulimitKeys, k)
	}

	command := fmt.Sprintf("systemctl cat containerd.service | grep -E -i '%s'", strings.Join(ulimitKeys, "|"))
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "0", execResult.exitCode, "validator command terminated with exit code %q but expected code 0", execResult.exitCode)

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

func execOnVMForScenario(ctx context.Context, s *Scenario, cmd string) *podExecResult {
	result, err := execOnVM(ctx, s.Runtime.Cluster.Kube, s.Runtime.VMPrivateIP, s.Runtime.DebugHostPod, string(s.Runtime.SSHKeyPrivate), cmd)
	require.NoError(s.T, err, "failed to execute command on VM")
	return result
}

func ValidateInstalledPackageVersion(ctx context.Context, s *Scenario, component, version string) {
	s.T.Logf("assert %s %s is installed on the VM", component, version)
	installedCommand := func() string {
		switch s.VHD.OS {
		case config.OSUbuntu:
			return "apt list --installed"
		case config.OSMariner, config.OSAzureLinux:
			return "dnf list installed"
		default:
			s.T.Fatalf("validator isn't implemented for OS %s", s.VHD.OS)
			return ""
		}
	}()
	execResult := execOnVMForScenario(ctx, s, installedCommand)
	require.Equal(s.T, "0", execResult.exitCode, "validator command terminated with exit code %q but expected code 0", execResult.exitCode)
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
	execResult := execOnVMForScenario(ctx, s, "cat /etc/default/kubelet")
	require.Equal(s.T, "0", execResult.exitCode, "validator command terminated with exit code %q but expected code 0", execResult.exitCode)

	// Search for "--node-ip" flag and its value.
	matches := regexp.MustCompile(`--node-ip=([a-zA-Z0-9.,]*)`).FindStringSubmatch(execResult.stdout.String())
	require.NotNil(s.T, matches, "could not find kubelet flag --node-ip")
	require.GreaterOrEqual(s.T, len(matches), 2, "could not find kubelet flag --node-ip")

	ipAddresses := strings.Split(matches[1], ",") // Could be multiple for dual-stack.
	require.GreaterOrEqual(s.T, len(ipAddresses), 1, "expected at least one --node-ip address, but got none")
	require.LessOrEqual(s.T, len(ipAddresses), 2, "expected at most two --node-ip addresses, but got %d", len(ipAddresses))

	// Check that each IP is a valid address.
	for _, ipAddress := range ipAddresses {
		require.NotNil(s.T, net.ParseIP(ipAddress), "--node-ip value %q is not a valid IP address", ipAddress)
	}
}

func ValidateIMDSRestrictionRule(ctx context.Context, s *Scenario, table string) {
	cmd := fmt.Sprintf("iptables -t %s -S | grep -q 'AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature'", table)
	execResult := execOnVMForScenario(ctx, s, cmd)
	require.Equal(s.T, "0", execResult.exitCode, "expected to find IMDS restriction rule, but did not")
}

func ValidateContainerdWASMShims(ctx context.Context, s *Scenario) {
	execResult := execOnVMForScenario(ctx, s, "cat /etc/containerd/config.toml")
	require.Equal(s.T, "0", execResult.exitCode, "expected to find containerd config.toml, but did not")
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
	command := "journalctl -u kubelet"
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "0", execResult.exitCode, "expected to find kubelet service logs, but did not")
	assert.NotContains(s.T, execResult.stdout.String(), "Stopped Kubelet")
	assert.Contains(s.T, execResult.stdout.String(), "Started Kubelet")
}

func ValidateServicesDoNotRestartKubelet(ctx context.Context, s *Scenario) {
	// grep all filesin /etc/systemd/system/ for /restart\s+kubelet/ and count results
	command := "grep -rl 'restart[[:space:]]\\+kubelet' /etc/systemd/system/"
	execResult := execOnVMForScenario(ctx, s, command)
	require.Equal(s.T, "1", execResult.exitCode, "expected to find no services containing 'restart kubelet' in /etc/systemd/system/, but found %q", execResult.stdout.String())
}

// ValidateKubeletHasFlags checks kubelet is started with the right flags and configs.
func ValidateKubeletHasFlags(ctx context.Context, s *Scenario, filePath string) {
	execResult := execOnVMForScenario(ctx, s, `journalctl -u kubelet`)
	require.Equal(s.T, "0", execResult.exitCode, "expected to find kubelet service logs, but did not")
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
	// assert that /etc/containerd/config.toml exists and does not contain deprecated properties from 1.7
	ValidateFileExcludesContent(ctx, s, "/etc/containerd/config.toml", "CriuPath", "CriuPath")
	// assert that containerd.server service file does not contain LimitNOFILE
	// https://github.com/containerd/containerd/blob/main/docs/containerd-2.0.md#limitnofile-configuration-has-been-removed
	ValidateFileExcludesContent(ctx, s, "/etc/systemd/system/containerd.service", "LimitNOFILE", "LimitNOFILE")
}

func ValidateRunc1_2Properties(ctx context.Context, s *Scenario, versions []string) {
	require.Lenf(s.T, versions, 1, "Expected exactly one version for moby-runc but got %d", len(versions))
	// assert versions[0] value starts with '1.2.'
	require.Truef(s.T, strings.HasPrefix(versions[0], "1.2."), "expected moby-runc version to start with '1.2.', got %v", versions[0])
	ValidateInstalledPackageVersion(ctx, s, "moby-runc", versions[0])
}

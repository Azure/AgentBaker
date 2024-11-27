package e2e

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/stretchr/testify/require"
)

func DirectoryValidator(path string, files []string) *LiveVMValidator {
	return &LiveVMValidator{
		Description: fmt.Sprintf("assert %s contents", path),
		Command:     fmt.Sprintf("ls -la %s", path),
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
			}
			for _, file := range files {
				if !strings.Contains(stdout, file) {
					return fmt.Errorf("expected to find file %s within directory %s, but did not", file, path)
				}
			}
			return nil
		},
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

func NvidiaSMINotInstalledValidator() *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert nvidia-smi is not installed",
		Command:     "nvidia-smi",
		Asserter: func(code, stdout, stderr string) error {
			if code != "1" {
				return fmt.Errorf(
					"nvidia-smi not installed should trigger exit 1, actual was: %q, stdout: %q, stderr: %q",
					code,
					stdout,
					stderr,
				)
			}
			if !strings.Contains(stderr, "nvidia-smi: command not found") {
				return fmt.Errorf(
					"expected stderr to contain 'nvidia-smi: command not found', actual: %q, stdout: %q",
					stderr,
					stdout,
				)
			}
			return nil
		},
	}
}

func NvidiaSMIInstalledValidator() *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert nvidia-smi is installed",
		Command:     "nvidia-smi",
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf(
					"nvidia-smi installed should trigger exit 0 actual was: %q, stdout: %q, stderr: %q",
					code,
					stdout,
					stderr,
				)
			}
			return nil
		},
	}
}

func NvidiaModProbeInstalledValidator() *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert nvidia-modprobe is installed",
		Command:     "nvidia-modprobe",
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf(
					"nvidia-modprobe installed should trigger exit 0 actual was: %q, stdout: %q, stderr: %q",
					code,
					stdout,
					stderr,
				)
			}
			return nil
		},
	}
}

func NonEmptyDirectoryValidator(dirName string) *LiveVMValidator {
	return &LiveVMValidator{
		Description: fmt.Sprintf("assert that there are files in %s", dirName),
		Command:     fmt.Sprintf("ls -1q %s | grep -q '^.*$' && true || false", dirName),
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("expected to find a file in directory %s, but did not", dirName)
			}
			return nil
		},
	}
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

func FileExcludesContentsValidator(fileName string, contents string, contentsName string) *LiveVMValidator {
	return &LiveVMValidator{
		Description: fmt.Sprintf("assert %s does not contain %s", fileName, contentsName),
		Command:     fmt.Sprintf("grep -q -F '%s' '%s'", contents, fileName),
		Asserter: func(code, stdout, stderr string) error {
			if code == "0" {
				return fmt.Errorf("expected to find a file '%s' without %s but did not", fileName, contentsName)
			}
			return nil
		},
	}
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

func execOnVMForScenario(ctx context.Context, s *Scenario, cmd string) *podExecResult {
	// TODO: cache it
	vmPrivateIP, err := getVMPrivateIPAddress(ctx, s)
	require.NoError(s.T, err, "failed to get VM private IP address")
	hostPodName, err := getHostNetworkDebugPodName(ctx, s.Runtime.Cluster.Kube, s.T)
	require.NoError(s.T, err, "failed to get host network debug pod name")
	result, err := execOnVM(ctx, s.Runtime.Cluster.Kube, vmPrivateIP, hostPodName, string(s.Runtime.SSHKeyPrivate), cmd, false)
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

// K8s 1.29+ should set --node-ip in kubelet flags due to behavior change in
// https://github.com/kubernetes/kubernetes/pull/121028
func kubeletNodeIPValidator() *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert /etc/default/kubelet has --node-ip flag set",
		Command:     "cat /etc/default/kubelet",
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
			}

			// Search for "--node-ip" flag and its value.
			matches := regexp.MustCompile(`--node-ip=([a-zA-Z0-9.,]*)`).FindStringSubmatch(stdout)
			if matches == nil || len(matches) < 2 {
				return fmt.Errorf("could not find kubelet flag --node-ip")
			}

			ipAddresses := strings.Split(matches[1], ",") // Could be multiple for dual-stack.
			if len(ipAddresses) == 0 || len(ipAddresses) > 2 {
				return fmt.Errorf("expected one or two --node-ip addresses, but got %d", len(ipAddresses))
			}

			// Check that each IP is a valid address.
			for _, ipAddress := range ipAddresses {
				if parsedIP := net.ParseIP(ipAddress); parsedIP == nil {
					return fmt.Errorf("--node-ip value %q is not a valid IP address", ipAddress)
				}
			}

			return nil
		},
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

// Ensure kubelet does not restart which can result in delays deploying pods and unnecessary nodepool scaling while the node is incapacitated.
// This is intended to stop services (e.g. nvidia-modprobe), restarting kubelet rather than specifying the dependency order to run before kubelet.service
func KubeletHasNotStoppedValidator() *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert that kubelet has not stopped or restarted",
		Command:     "journalctl -u kubelet",
		Asserter: func(code, stdout, stderr string) error {
			startedText := "Started Kubelet"
			stoppedText := "Stopped Kubelet"
			stoppedCount := strings.Count(stdout, stoppedText)
			startedCount := strings.Count(stdout, startedText)
			if stoppedCount > 0 {
				return fmt.Errorf("expected no occurences of '%s' in kubelet log, but found %d", stoppedText, stoppedCount)
			}
			if startedCount == 0 {
				return fmt.Errorf("expected at least one occurence of '%s' in kubelet log, but found 0", startedText)
			}
			return nil
		},
	}
}

// ValidateKubeletHasFlags checks kubelet is started with the right flags and configs.
func ValidateKubeletHasFlags(ctx context.Context, s *Scenario, filePath string) {
	execResult := execOnVMForScenario(ctx, s, `journalctl -u kubelet`)
	require.Equal(s.T, "0", execResult.exitCode, "expected to find kubelet service logs, but did not")
	configFileFlags := fmt.Sprintf("FLAG: --config=\"%s\"", filePath)
	require.Containsf(s.T, execResult.stdout.String(), configFileFlags, "expected to find flag %s, but not found", "config")
}

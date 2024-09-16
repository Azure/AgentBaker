package e2e

import (
	"fmt"
	"net"
	"regexp"
	"strings"
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

func SysctlConfigValidator(customSysctls map[string]string) *LiveVMValidator {
	keysToCheck := make([]string, len(customSysctls))
	for k := range customSysctls {
		keysToCheck = append(keysToCheck, k)
	}
	// regex used in sed command to remove extra spaces between two numerical values, used to verify correct values for
	// sysctls that have string values, e.g. net.ipv4.ip_local_port_range, which would be printed with extra spaces
	return &LiveVMValidator{
		Description: "assert sysctl settings",
		Command:     fmt.Sprintf("sysctl %s | sed -E 's/([0-9])\\s+([0-9])/\\1 \\2/g'", strings.Join(keysToCheck, " ")),
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
			}
			for name, value := range customSysctls {
				if !strings.Contains(stdout, fmt.Sprintf("%s = %v", name, value)) {
					return fmt.Errorf("expected to find %s set to %v, but was not", name, value)
				}
			}
			return nil
		},
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

func FileHasContentsValidator(fileName string, contents string) *LiveVMValidator {
	steps := []string{
		fmt.Sprintf("ls -la %[1]s", fileName),
		fmt.Sprintf("(sudo cat %[1]s | grep -q %[2]q)", fileName, contents),
	}

	command := makeExecutableCommand(steps)

	return &LiveVMValidator{
		Description: fmt.Sprintf("Assert that %s has defined contents", fileName),
		// on mariner and ubuntu, the chronyd drop-in file is not readable by the default user, so we run as root.
		Command: command,
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("expected to find a file '%s' with contents '%s' but did not", fileName, contents)
			}
			return nil
		},
	}
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

func ServiceCanRestartValidator(serviceName string, restartTimeoutInSeconds int) *LiveVMValidator {
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

	return &LiveVMValidator{
		Description: fmt.Sprintf("asserts that %s restarts after it is killed", serviceName),
		Command:     command,
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("service kill and check terminated with exit code %q (expected 0).\nCommand: %s\n\nStdout:\n%s\n\n Stderr:\n%s", code, command, stdout, stderr)
			}
			return nil
		},
	}
}

func CommandHasOutputValidator(commandToExecute string, expectedOutput string) *LiveVMValidator {
	steps := []string{
		fmt.Sprint(commandToExecute),
	}

	command := makeExecutableCommand(steps)

	return &LiveVMValidator{
		Description: fmt.Sprintf("asserts that %s has output %s", commandToExecute, expectedOutput),
		Command:     command,
		Asserter: func(code, stdout, stderr string) error {
			if !strings.Contains(stderr, expectedOutput) {
				return fmt.Errorf("'%s' output did not contain expected string Stdout:\n%s\n\n Stderr:\n%s", command, stdout, stderr)
			}
			if code != "0" {
				return fmt.Errorf("command failed with exit code %q (expected 0).\nCommand: %s\n\nStdout:\n%s\n\n Stderr:\n%s", code, command, stdout, stderr)
			}
			return nil
		},
	}
}

func UlimitValidator(ulimits map[string]string) *LiveVMValidator {
	ulimitKeys := make([]string, 0, len(ulimits))
	for k := range ulimits {
		ulimitKeys = append(ulimitKeys, k)
	}

	return &LiveVMValidator{
		Description: "assert ulimit settings",
		Command:     fmt.Sprintf("systemctl cat containerd.service | grep -E -i '%s'", strings.Join(ulimitKeys, "|")),
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
			}
			for name, value := range ulimits {
				if !strings.Contains(stdout, fmt.Sprintf("%s=%v", name, value)) {
					return fmt.Errorf(fmt.Sprintf("expected to find %s set to %v, but was not", name, value))
				}
			}
			return nil
		},
	}
}

func containerdVersionValidator(version string) *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert containerd version",
		Command:     "containerd --version",
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
			}

			if !strings.Contains(stdout, version) {
				return fmt.Errorf(fmt.Sprintf("expected to find containerd version %s, got: %s", version, stdout))
			}
			return nil
		},
	}
}

func runcVersionValidator(version string) *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert runc version",
		Command:     "runc --version",
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
			}

			// runc output
			if !strings.Contains(stdout, "runc version "+version) {
				return fmt.Errorf(fmt.Sprintf("expected to find runc version %s, got: %s", version, stdout))
			}
			return nil
		},
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

func imdsRestrictionRuleValidator(table string) *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert that the IMDS restriction rule is present",
		Command:     fmt.Sprintf("iptables -t %s -S | grep -q 'AKS managed: added by AgentBaker ensureIMDSRestriction for IMDS restriction feature'", table),
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("expected to find the IMDS restriction rule, but did not")
			}
			return nil
		},
	}
}

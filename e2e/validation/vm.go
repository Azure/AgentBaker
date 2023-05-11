package validation

import (
	"fmt"
	"strings"
)

func CommonLiveVMValidators() []*LiveVMValidator {
	return []*LiveVMValidator{
		{
			Description: "assert /etc/default/kubelet should not contain dynamic config dir flag",
			Command:     "cat /etc/default/kubelet",
			Asserter: func(code, stdout, stderr string) error {
				if code != "0" {
					return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
				}
				if strings.Contains(stdout, "--dynamic-config-dir") {
					return fmt.Errorf("/etc/default/kubelet should not contain kubelet flag '--dynamic-config-dir', but does")
				}
				return nil
			},
		},
		SysctlConfigValidator(
			map[string]int{
				"net.ipv4.tcp_retries2":             8,
				"net.core.message_burst":            80,
				"net.core.message_cost":             40,
				"net.core.somaxconn":                16384,
				"net.ipv4.tcp_max_syn_backlog":      16384,
				"net.ipv4.neigh.default.gc_thresh1": 4096,
				"net.ipv4.neigh.default.gc_thresh2": 8192,
				"net.ipv4.neigh.default.gc_thresh3": 16384,
			},
		),
		DirectoryValidator(
			"/var/log/azure/aks",
			[]string{
				"cluster-provision.log",
				"cluster-provision-cse-output.log",
				"cloud-init-files.paved",
				"vhd-install.complete",
				"cloud-config.txt",
			},
		),
	}
}

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
					return fmt.Errorf(fmt.Sprintf("expected to find file %s within directory %s, but did not", file, path))
				}
			}
			return nil
		},
	}
}

func SysctlConfigValidator(customSysctls map[string]int) *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert sysctl settings",
		Command:     "sysctl -a",
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
			}
			for name, value := range customSysctls {
				if !strings.Contains(stdout, fmt.Sprintf("%s = %d", name, value)) {
					return fmt.Errorf(fmt.Sprintf("expected to find %s set to %d, but was not", name, value))
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

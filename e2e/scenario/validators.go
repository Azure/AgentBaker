package scenario

import (
	"encoding/json"
	"fmt"
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
					return fmt.Errorf(fmt.Sprintf("expected to find file %s within directory %s, but did not", file, path))
				}
			}
			return nil
		},
	}
}

func SysctlConfigValidator(customSysctls map[string]string) *LiveVMValidator {
	keysToCheck := make([]string, 0)
	for k, _ := range customSysctls {
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
					return fmt.Errorf(fmt.Sprintf("expected to find %s set to %v, but was not", name, value))
				}
			}
			return nil
		},
	}
}

func KubeletConfigValidator(kubeletConfigParams map[string]string) *LiveVMValidator {
	keysToCheck := make([]string, 0)
	for k, _ := range kubeletConfigParams {
		keysToCheck = append(keysToCheck, k)
	}
	return &LiveVMValidator{
		Description: "assert kubelet config settings",
		Command:     fmt.Sprintf("jq `{%s}` /etc/default/kubeleconfig.json", strings.Join(keysToCheck, ",")),
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
			}
			// this works fine for checking simple unnested parameters, will not really work for nested structures
			var kubeletConfig map[string]interface{}
			err := json.Unmarshal([]byte(stdout), &kubeletConfig)
			if err != nil {
				return err
			}
			for kubeletParam, expectedValue := range kubeletConfigParams {
				if kubeletConfig[kubeletParam] != expectedValue {
					return fmt.Errorf(fmt.Sprintf("expected to find %s set to %v, but was not", kubeletParam, expectedValue))
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

func NonEmptyDirectoryValidator(dirName string) *LiveVMValidator {
	return &LiveVMValidator{
		Description: "assert that there are files in directory",
		Command:     fmt.Sprintf("ls -1q %s | grep -q '^.*$' && true || false", dirName),
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("expected to find a file in directory %s, but did not", dirName)
			}
			return nil
		},
	}
}

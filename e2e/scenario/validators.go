package scenario

import (
	"fmt"
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
					return fmt.Errorf(fmt.Sprintf("expected to find file %s within directory %s, but did not", file, path))
				}
			}
			return nil
		},
	}
}

func SysctlConfigValidator(customSysctls map[string]string) *LiveVMValidator {
	keysToCheck := make([]string, len(customSysctls))
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

// KubenetEnsureNoDupEbtablesValidator checks that ebtables rules were installed by
// the ensure-no-dup.sh script to block duplicate packets from the promiscuous bridge.
// This assumes at least one pod (without hostNetwork) has already run on the node.
func KubenetEnsureNoDupEbtablesValidator() *LiveVMValidator {
	// Use regex match for the rules because the MAC and IP addresses can vary.
	expectedRulePatterns := []string{
		`-j AKS-DEDUP-PROMISC`,
		`-p IPv4 -s [0-9a-f:]+ -o veth\+ --ip-src [0-9.]+ -j ACCEPT`,
		`-p IPv4 -s [0-9a-f:]+ -o veth\+ --ip-src [0-9.]+/[0-9]+ -j DROP`,
	}
	regexes := make(map[string]*regexp.Regexp, len(expectedRulePatterns))
	for _, s := range expectedRulePatterns {
		regexes[s] = regexp.MustCompile(s)
	}

	return &LiveVMValidator{
		Description: "assert kubenet ensure-no-dup ebtables rules",
		// Grep matches rules with "-" at start of line.
		// This command will fail and be retried to account for delay between
		// when the CNI creates the bridge and when the ensure-no-dup systemd unit completes.
		Command: fmt.Sprintf(`ebtables -L | grep "^-"`),
		Asserter: func(code, stdout, stderr string) error {
			if code != "0" {
				return fmt.Errorf("validator command terminated with exit code %q but expected code 0", code)
			}

			for pattern, re := range regexes {
				if !re.MatchString(stdout) {
					return fmt.Errorf("could not find expected ebtables rule matching pattern %q", pattern)
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
				return fmt.Errorf(fmt.Sprintf("expected to find containerd version %s, but was not", version))
			}
			return nil
		},
	}
}

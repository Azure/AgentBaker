package scenario

import (
	"fmt"
	"strings"
)

func DirectoryValidator(path string, files []string) *LiveVMValidator {
	return &LiveVMValidator{
		Description: fmt.Sprintf("assert %s contents", path),
		Command:     fmt.Sprintf("ls -la %s", path),
		Asserter: func(stdout, stderr string) error {
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
		Asserter: func(stdout, stderr string) error {
			for name, value := range customSysctls {
				if !strings.Contains(stdout, fmt.Sprintf("%s = %d", name, value)) {
					return fmt.Errorf(fmt.Sprintf("expected to find %s set to %d, but was not", name, value))
				}
			}
			return nil
		},
	}
}

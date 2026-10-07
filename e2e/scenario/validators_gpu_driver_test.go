// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package scenario

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNvidiaDriverVersionValidationScript(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "sudo"),
		[]byte("#!/usr/bin/env bash\nexec \"$@\"\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "nvidia-smi"),
		[]byte("#!/usr/bin/env bash\nprintf '%s\\n' \"$TEST_DRIVER_VERSIONS\"\nexit \"$TEST_SMI_EXIT_CODE\"\n"), 0o755))

	cases := []struct {
		name     string
		expected string
		versions string
		smiExit  string
		wantErr  bool
	}{
		{"matching LTS", "580.178.04", "580.178.04", "0", false},
		{"matching v20", "595.91.07", "595.91.07", "0", false},
		{"wrong branch", "580.178.04", "595.91.07", "0", true},
		{"stale LTS patch", "580.178.04", "580.126.09", "0", true},
		{"matching multiple GPUs", "595.91.07", "595.91.07\n595.91.07", "0", false},
		{"second GPU mismatches", "595.91.07", "595.91.07\n580.178.04", "0", true},
		{"empty GPU output", "580.178.04", "", "0", true},
		{"nvidia-smi failure", "580.178.04", "580.178.04", "1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script, err := nvidiaDriverVersionValidationScript(tc.expected)
			require.NoError(t, err)
			scriptPath := filepath.Join(t.TempDir(), "validate-driver.sh")
			require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o755))
			command := exec.Command(scriptPath)
			command.Env = append(os.Environ(),
				"PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"),
				"TEST_DRIVER_VERSIONS="+tc.versions,
				"TEST_SMI_EXIT_CODE="+tc.smiExit)
			output, err := command.CombinedOutput()
			if tc.wantErr {
				require.Error(t, err, string(output))
			} else {
				require.NoError(t, err, string(output))
			}
		})
	}
}

func TestNvidiaDriverVersionValidationScriptRejectsInvalidVersion(t *testing.T) {
	for _, expected := range []string{"", "580", "580.178.04 invalid"} {
		t.Run(expected, func(t *testing.T) {
			script, err := nvidiaDriverVersionValidationScript(expected)
			require.Error(t, err)
			require.Empty(t, script)
		})
	}
}

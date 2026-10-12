package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomerNvidiaDriverValidationRequiresStagedInstaller(t *testing.T) {
	const expectedVersion = "580.159.04"
	binDir := t.TempDir()
	installerPath := filepath.Join(t.TempDir(), "nvidia-installer")
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "lsmod"), []byte("#!/bin/sh\nprintf 'nvidia 123 0\\n'\n"), 0o755))
	sudo := fmt.Sprintf(`#!/bin/sh
case "$1" in
  modinfo) printf '%%s\n' '%s'; exit 0 ;;
  nvidia-smi)
    if [ "$2" = "--query-gpu=driver_version" ]; then
      printf '%%s\n' '%s'
    fi
    exit 0
    ;;
esac
exit 1
`, expectedVersion, expectedVersion)
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "sudo"), []byte(sudo), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	runValidation := func() error {
		output, err := exec.Command("bash", "-x", "-c", customerNvidiaDriverValidationScript(expectedVersion, installerPath)).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, output)
		}
		return nil
	}
	require.Error(t, runValidation(), "validation must reject correct driver state when the staged installer was removed")

	require.NoError(t, os.WriteFile(installerPath, []byte("#!/bin/sh\n"), 0o755))
	require.NoError(t, runValidation(), "validation must pass when the staged installer and driver state are present")
}

func TestValidateSysctlOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected map[string]string
		wantErr  bool
	}{
		{
			name:     "empty value",
			output:   "net.ipv4.ip_local_reserved_ports = \n",
			expected: map[string]string{"net.ipv4.ip_local_reserved_ports": ""},
		},
		{
			name:     "nonempty value does not match empty",
			output:   "net.ipv4.ip_local_reserved_ports = 65330\n",
			expected: map[string]string{"net.ipv4.ip_local_reserved_ports": ""},
			wantErr:  true,
		},
		{
			name:     "missing empty value",
			output:   "net.ipv4.ip_local_port_range = 32768\t65535\n",
			expected: map[string]string{"net.ipv4.ip_local_reserved_ports": ""},
			wantErr:  true,
		},
		{
			name:   "multiple values with kernel whitespace",
			output: "net.ipv4.ip_local_port_range = 32768\t65535\nnet.netfilter.nf_conntrack_max = 2097152\n",
			expected: map[string]string{
				"net.ipv4.ip_local_port_range":   "32768 65535",
				"net.netfilter.nf_conntrack_max": "2097152",
			},
		},
		{
			name:     "numeric prefix does not match",
			output:   "net.netfilter.nf_conntrack_max = 20971520\n",
			expected: map[string]string{"net.netfilter.nf_conntrack_max": "2097152"},
			wantErr:  true,
		},
		{
			name:     "port list prefix does not match",
			output:   "net.ipv4.ip_local_reserved_ports = 65330,65331\n",
			expected: map[string]string{"net.ipv4.ip_local_reserved_ports": "65330"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSysctlOutput(tt.output, tt.expected)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWindowsFileContainsBootstrapTokenScriptDoesNotExposeToken(t *testing.T) {
	const token = "bake00.0123456789abcdef"

	script, err := windowsFileContainsBootstrapTokenScript(`C:\AzureData\CustomDataSetupScript.ps1`, token)
	require.NoError(t, err)
	require.NotContains(t, script, token)

	hash := sha256.Sum256([]byte(token))
	require.Contains(t, script, hex.EncodeToString(hash[:]))
	for _, marker := range []string{
		windowsScanAbsentMarker,
		windowsScanPresentMarker,
		windowsScanFileMissingMarker,
		windowsScanErrorMarker,
	} {
		require.Contains(t, script, marker)
	}
}

func TestParseWindowsContentScanResult(t *testing.T) {
	tests := []struct {
		name          string
		result        *podExecResult
		containsToken bool
		wantErr       bool
	}{
		{
			name:   "absent marker with success",
			result: &podExecResult{exitCode: "0", stdout: windowsScanAbsentMarker},
		},
		{
			name:          "present marker with collapsed Windows SSH exit code",
			result:        &podExecResult{exitCode: "1", stdout: windowsScanPresentMarker},
			containsToken: true,
		},
		{
			name:    "runtime error marker",
			result:  &podExecResult{exitCode: "1", stdout: windowsScanErrorMarker},
			wantErr: true,
		},
		{
			name:    "file missing marker",
			result:  &podExecResult{exitCode: "1", stdout: windowsScanFileMissingMarker},
			wantErr: true,
		},
		{
			name:    "missing marker",
			result:  &podExecResult{exitCode: "0"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containsToken, err := parseWindowsContentScanResult(tt.result)
			require.Equal(t, tt.containsToken, containsToken)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

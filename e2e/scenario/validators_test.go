package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/logging"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestValidateVulnerableKernelModulesDisabledAzureLinux(t *testing.T) {
	for _, tc := range []struct {
		distro datamodel.Distro
		absent bool
	}{
		{datamodel.AKSAzureLinuxV3Gen2, true},
		{datamodel.AKSAzureLinuxV3Gen2Kata, false},
		{datamodel.AKSAzureLinuxV3OSGuardGen2FIPSTL, false},
		{datamodel.CustomizedImageKata, false},
		{datamodel.AKSAzureLinuxV2Gen2, false},
		{datamodel.AKSAzureLinuxV2Gen2Kata, false},
	} {
		t.Run(string(tc.distro), func(t *testing.T) {
			s := &Scenario{
				Config:  Config{VHD: &config.Image{OS: config.OSAzureLinux, Distro: tc.distro}},
				Runtime: &ScenarioRuntime{},
			}
			err := ValidateVulnerableKernelModulesDisabled(logging.WithLogger(t.Context(), t), s)
			// No VM is needed: the contextual error identifies the selected validation path.
			require.ErrorContains(t, err, "cannot execute script on a nil VM")
			if tc.absent {
				require.ErrorContains(t, err, "check that the AzureLinux 3.0 modprobe blacklist is correctly scoped")
			} else {
				require.ErrorContains(t, err, "validate vulnerable kernel module mitigation")
			}
		})
	}
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

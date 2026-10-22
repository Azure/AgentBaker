package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/stretchr/testify/require"
	resourcev1 "k8s.io/api/resource/v1"
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

func TestValidateDRAMIGAllocation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*resourcev1.ResourceClaim, *resourcev1.ResourceSliceList)
		wantErr string
	}{
		{name: "matching allocated MIG device"},
		{
			name: "fully qualified attributes",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				attrs := slices.Items[0].Spec.Devices[0].Attributes
				attrs["gpu.nvidia.com/type"] = attrs["type"]
				attrs["gpu.nvidia.com/profile"] = attrs["profile"]
				delete(attrs, "type")
				delete(attrs, "profile")
			},
		},
		{
			name: "missing allocation",
			mutate: func(claim *resourcev1.ResourceClaim, _ *resourcev1.ResourceSliceList) {
				claim.Status.Allocation = nil
			},
			wantErr: "no allocation",
		},
		{
			name: "empty allocation",
			mutate: func(claim *resourcev1.ResourceClaim, _ *resourcev1.ResourceSliceList) {
				claim.Status.Allocation.Devices.Results = nil
			},
			wantErr: "exactly one allocated device",
		},
		{
			name: "multiple allocated devices",
			mutate: func(claim *resourcev1.ResourceClaim, _ *resourcev1.ResourceSliceList) {
				results := &claim.Status.Allocation.Devices.Results
				*results = append(*results, (*results)[0])
			},
			wantErr: "exactly one allocated device",
		},
		{
			name: "wrong request",
			mutate: func(claim *resourcev1.ResourceClaim, _ *resourcev1.ResourceSliceList) {
				claim.Status.Allocation.Devices.Results[0].Request = "other"
			},
			wantErr: `got request "other"`,
		},
		{
			name: "wrong allocated driver",
			mutate: func(claim *resourcev1.ResourceClaim, _ *resourcev1.ResourceSliceList) {
				claim.Status.Allocation.Devices.Results[0].Driver = "other.example.com"
			},
			wantErr: `from driver "other.example.com"`,
		},
		{
			name: "missing resource slices",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				slices.Items = nil
			},
			wantErr: "not found on node",
		},
		{
			name: "wrong slice driver",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				slices.Items[0].Spec.Driver = "other.example.com"
			},
			wantErr: "not found on node",
		},
		{
			name: "wrong pool",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				slices.Items[0].Spec.Pool.Name = "other-pool"
			},
			wantErr: "not found on node",
		},
		{
			name: "wrong node",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				slices.Items[0].Spec.NodeName = to.Ptr("other-node")
			},
			wantErr: "not found on node",
		},
		{
			name: "missing node",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				slices.Items[0].Spec.NodeName = nil
			},
			wantErr: "not found on node",
		},
		{
			name: "matching device exists but was not allocated",
			mutate: func(claim *resourcev1.ResourceClaim, _ *resourcev1.ResourceSliceList) {
				claim.Status.Allocation.Devices.Results[0].Device = "other-device"
			},
			wantErr: "not found on node",
		},
		{
			name: "full GPU instead of MIG",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				slices.Items[0].Spec.Devices[0].Attributes["type"] = resourcev1.DeviceAttribute{StringValue: to.Ptr("gpu")}
			},
			wantErr: `has type "gpu", expected "mig"`,
		},
		{
			name: "wrong MIG profile",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				slices.Items[0].Spec.Devices[0].Attributes["profile"] = resourcev1.DeviceAttribute{StringValue: to.Ptr("2g.20gb")}
			},
			wantErr: `has profile "2g.20gb", expected "1g.10gb"`,
		},
		{
			name: "missing type",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				delete(slices.Items[0].Spec.Devices[0].Attributes, "type")
			},
			wantErr: `no string attribute "type"`,
		},
		{
			name: "missing profile",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				delete(slices.Items[0].Spec.Devices[0].Attributes, "profile")
			},
			wantErr: `no string attribute "profile"`,
		},
		{
			name: "non-string profile",
			mutate: func(_ *resourcev1.ResourceClaim, slices *resourcev1.ResourceSliceList) {
				slices.Items[0].Spec.Devices[0].Attributes["profile"] = resourcev1.DeviceAttribute{IntValue: to.Ptr(int64(1))}
			},
			wantErr: `no string attribute "profile"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claim := &resourcev1.ResourceClaim{
				Status: resourcev1.ResourceClaimStatus{
					Allocation: &resourcev1.AllocationResult{
						Devices: resourcev1.DeviceAllocationResult{
							Results: []resourcev1.DeviceRequestAllocationResult{{
								Request: "gpu", Driver: "gpu.nvidia.com", Pool: "scenario-node", Device: "mig-device",
							}},
						},
					},
				},
			}
			slices := &resourcev1.ResourceSliceList{
				Items: []resourcev1.ResourceSlice{{
					Spec: resourcev1.ResourceSliceSpec{
						Driver:   "gpu.nvidia.com",
						Pool:     resourcev1.ResourcePool{Name: "scenario-node"},
						NodeName: to.Ptr("scenario-node"),
						Devices: []resourcev1.Device{{
							Name: "mig-device",
							Attributes: map[resourcev1.QualifiedName]resourcev1.DeviceAttribute{
								"type":    {StringValue: to.Ptr("mig")},
								"profile": {StringValue: to.Ptr("1g.10gb")},
							},
						}},
					},
				}},
			}
			if tt.mutate != nil {
				tt.mutate(claim, slices)
			}
			err := validateDRAMIGAllocation(claim, slices, "scenario-node", "1g.10gb")
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
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

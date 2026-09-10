package e2e

import (
	"fmt"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestValidateRxBufferDefaultDoesNotSkipBySKU(t *testing.T) {
	for _, distro := range []datamodel.Distro{
		datamodel.AKSAzureLinuxV3Gen2,
		datamodel.AKSACLGen2TL,
		datamodel.AKSUbuntuContainerd2404Gen2,
	} {
		for _, sku := range []string{"Standard_NM16ads_MA35D", "Standard_D2ds_v5", "Standard_D2ds_v6", "Standard_D2ds_v10"} {
			t.Run(string(distro)+"/"+sku, func(t *testing.T) {
				s := &Scenario{
					Config: Config{VHD: &config.Image{Distro: distro}},
					Logger: t,
					Runtime: &ScenarioRuntime{
						NBC: &datamodel.NodeBootstrappingConfiguration{
							AgentPoolProfile: &datamodel.AgentPoolProfile{VMSize: sku},
						},
					},
				}
				err := ValidateRxBufferDefault(t.Context(), s)
				require.ErrorContains(t, err, "cannot execute script on a nil VM")
			})
		}
	}
}

func TestParseCurrentRxBuffer(t *testing.T) {
	for _, tc := range []struct {
		name, output, want string
		wantError          bool
	}{
		{
			name: "current not maximum",
			output: "Ring parameters for eth1:\nPre-set maximums:\nRX:\t8192\nTX:\t16384\n" +
				"Current hardware settings:\nRX:\t512\nRX Mini:\t0\nRX Jumbo:\t0\nTX:\t256\n",
			want: "512",
		},
		{
			name:   "whitespace and other RX fields",
			output: "Current hardware settings:\nRX Mini: 0\nRX Jumbo: 0\n  RX:\t2048  \nTX: 256\n",
			want:   "2048",
		},
		{name: "missing current section", output: "Pre-set maximums:\nRX: 8192\n", wantError: true},
		{name: "missing RX", output: "Current hardware settings:\nTX: 256\n", wantError: true},
		{name: "empty RX", output: "Current hardware settings:\nRX:\n", wantError: true},
		{name: "extra RX fields", output: "Current hardware settings:\nRX: 512 1024\n", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCurrentRxBuffer(tc.output)
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestValidateDefaultRxBufferSize(t *testing.T) {
	tests := []struct {
		cpus      int
		rx        string
		wantError bool
	}{
		{cpus: 2, rx: "512"},
		{cpus: 2, rx: "1024"},
		{cpus: 2, rx: "2048"},
		{cpus: 2, rx: "4096"},
		{cpus: 3, rx: "1024"},
		{cpus: 4, rx: "512"},
		{cpus: 4, rx: "1024", wantError: true},
		{cpus: 4, rx: "2048"},
		{cpus: 4, rx: "4096"},
		{cpus: 16, rx: "1024", wantError: true},
		{cpus: 16, rx: "2048"},
		{cpus: 2, rx: "", wantError: true},
		{cpus: 2, rx: "null", wantError: true},
		{cpus: 2, rx: "n/a", wantError: true},
		{cpus: 2, rx: "0", wantError: true},
		{cpus: 2, rx: "-512", wantError: true},
		{cpus: 0, rx: "1024", wantError: true},
		{cpus: -1, rx: "1024", wantError: true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%dCPUs/RX%s", tt.cpus, tt.rx), func(t *testing.T) {
			err := validateDefaultRxBufferSize(tt.cpus, tt.rx)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

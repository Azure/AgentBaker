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

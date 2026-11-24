package scenario

import (
	"context"
	"strconv"
	"testing"

	aksnodeconfigv1 "github.com/Azure/agentbaker/aks-node-controller/pkg/gen/aksnodeconfig/v1"
	"github.com/Azure/agentbaker/aks-node-controller/pkg/nodeconfigutils"
	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

func TestPrepareNodeConfigurationEnforcesSKUAfterBothMutatorPaths(t *testing.T) {
	for _, strict := range []bool{false, true} {
		t.Run(strconv.FormatBool(strict), func(t *testing.T) {
			original := config.Config
			config.Config = config.DefaultConfiguration()
			config.Config.AdHocSKUValidation = strict
			config.Config.DefaultVMSKU = "Standard_D16pds_v7"
			t.Cleanup(func() { config.Config = original })

			cluster := nodeConfigTestCluster("1.36.5")
			s := &Scenario{
				Config: Config{
					VHD: config.VHDUbuntu2404ArmContainerd,
					BootstrapConfigMutator: func(_ *Cluster, nbc *datamodel.NodeBootstrappingConfiguration) {
						nbc.AgentPoolProfile.VMSize = "Standard_D2pds_v5"
						nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize = "Standard_D2pds_v5"
					},
					AKSNodeConfigMutator: func(_ *Cluster, cfg *aksnodeconfigv1.Configuration) {
						cfg.VmSize = "Standard_D2pds_v5"
					},
					AKSNodeConfigMutatorWithError: func(_ context.Context, _ *Cluster, cfg *aksnodeconfigv1.Configuration) error {
						cfg.VmSize = "Standard_D2pds_v6"
						return nil
					},
				},
				Runtime: &ScenarioRuntime{Cluster: cluster},
			}
			nbc, err := getBaseNBC(t.Context(), cluster, s.VHD)
			require.NoError(t, err)
			require.NoError(t, prepareNodeConfiguration(t.Context(), s, nbc))
			wantNBC, wantANC := "Standard_D2pds_v5", "Standard_D2pds_v6"
			if strict {
				wantNBC, wantANC = config.Config.VMSKU(), config.Config.VMSKU()
			}
			require.Equal(t, wantNBC, nbc.AgentPoolProfile.VMSize)
			require.Equal(t, wantNBC, nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize)
			require.Equal(t, wantANC, s.Runtime.AKSNodeConfig.VmSize)
			rendered, err := nodeconfigutils.UnmarshalConfigurationV1([]byte(nbc.AKSNodeConfigJSON))
			require.NoError(t, err)
			require.Equal(t, wantANC, rendered.VmSize)
		})
	}
}

func TestOverrideBootstrapVMSize(t *testing.T) {
	for _, size := range []string{"Standard_E64adus_v7", "Standard_D16pds_v7"} {
		t.Run(size, func(t *testing.T) {
			nbc := &datamodel.NodeBootstrappingConfiguration{
				AgentPoolProfile: &datamodel.AgentPoolProfile{VMSize: "Standard_D2ds_v6"},
				ContainerService: &datamodel.ContainerService{
					Properties: &datamodel.Properties{
						AgentPoolProfiles: []*datamodel.AgentPoolProfile{
							{VMSize: "Standard_D2pds_v5"},
						},
					},
				},
			}
			require.NoError(t, overrideBootstrapVMSize(nbc, size))
			require.Equal(t, size, nbc.AgentPoolProfile.VMSize)
			require.Equal(t, size, nbc.ContainerService.Properties.AgentPoolProfiles[0].VMSize)
		})
	}
}

func TestOverrideBootstrapVMSizeRejectsIncompleteInputs(t *testing.T) {
	for _, nbc := range []*datamodel.NodeBootstrappingConfiguration{
		nil,
		{},
		{AgentPoolProfile: &datamodel.AgentPoolProfile{}},
		{
			AgentPoolProfile: &datamodel.AgentPoolProfile{},
			ContainerService: &datamodel.ContainerService{
				Properties: &datamodel.Properties{
					AgentPoolProfiles: []*datamodel.AgentPoolProfile{nil},
				},
			},
		},
	} {
		require.Error(t, overrideBootstrapVMSize(nbc, "Standard_D16pds_v7"))
	}
	require.Error(t, overrideBootstrapVMSize(&datamodel.NodeBootstrappingConfiguration{}, ""))
}

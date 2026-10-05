package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/stretchr/testify/require"
)

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

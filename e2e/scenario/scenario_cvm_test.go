package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCVMScenarioUsesWestEuropeAndDefaultValidation(t *testing.T) {
	s := newCVMScenario(
		"test",
		"description",
		config.VHDUbuntu2404CVMGen2Containerd,
		ClusterKubenet,
		cvmSNPSKU,
		cvmPlatformSEVSNP,
		false,
	)

	assert.Equal(t, cvmLocation, s.Location)
	assert.True(t, s.Tags.CVM)
	assert.Equal(t, config.VHDUbuntu2404CVMGen2Containerd, s.VHD)
	assert.False(t, s.SkipDefaultValidation)
	assert.NotNil(t, s.Validator)

	nbc := &datamodel.NodeBootstrappingConfiguration{
		AgentPoolProfile: &datamodel.AgentPoolProfile{},
	}
	s.BootstrapConfigMutator(nil, nbc)
	assert.Equal(t, cvmSNPSKU, nbc.AgentPoolProfile.VMSize)
}

func TestRegisteredCVMScenarios(t *testing.T) {
	expected := map[string]struct {
		vhd     *config.Image
		useNVMe bool
	}{
		"Ubuntu2004_CVM_SEVSNP":        {vhd: config.VHDUbuntu2004CVMGen2Containerd},
		"Ubuntu2404_CVM_SEVSNP":        {vhd: config.VHDUbuntu2404CVMGen2Containerd},
		"AzureLinuxV3_CVM_SEVSNP":      {vhd: config.VHDAzureLinuxV3CVMGen2},
		"Ubuntu2604Minimal_CVM_SEVSNP": {vhd: config.VHDUbuntu2604MinimalCVMGen2Containerd},
		"Ubuntu2604Minimal_CVM_TDX":    {vhd: config.VHDUbuntu2604MinimalCVMGen2Containerd, useNVMe: true},
	}

	var found int
	for _, s := range List() {
		if !s.Tags.CVM {
			continue
		}
		found++
		want, ok := expected[s.Name]
		require.True(t, ok, "unexpected CVM scenario %q", s.Name)
		assert.Equal(t, cvmLocation, s.Location)
		assert.Equal(t, want.vhd, s.VHD)
		assert.Equal(t, want.useNVMe, s.UseNVMe)
		assert.False(t, s.SkipDefaultValidation)
		assert.NotNil(t, s.Validator)
	}
	assert.Equal(t, len(expected), found)
}

func TestConfigureConfidentialVM(t *testing.T) {
	vmss := &armcompute.VirtualMachineScaleSet{
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				OSProfile: &armcompute.VirtualMachineScaleSetOSProfile{},
			},
		},
	}

	configureConfidentialVM(vmss, cvmTDXSKU)

	require.NotNil(t, vmss.SKU)
	require.NotNil(t, vmss.SKU.Name)
	assert.Equal(t, cvmTDXSKU, *vmss.SKU.Name)

	require.NotNil(t, vmss.Properties)
	require.NotNil(t, vmss.Properties.VirtualMachineProfile)
	profile := vmss.Properties.VirtualMachineProfile
	assert.Nil(t, profile.OSProfile)
	require.NotNil(t, profile.SecurityProfile)
	require.NotNil(t, profile.SecurityProfile.SecurityType)
	assert.Equal(t, armcompute.SecurityTypesConfidentialVM, *profile.SecurityProfile.SecurityType)
	require.NotNil(t, profile.SecurityProfile.EncryptionAtHost)
	assert.True(t, *profile.SecurityProfile.EncryptionAtHost)
	require.NotNil(t, profile.SecurityProfile.UefiSettings)
	require.NotNil(t, profile.SecurityProfile.UefiSettings.SecureBootEnabled)
	assert.True(t, *profile.SecurityProfile.UefiSettings.SecureBootEnabled)
	require.NotNil(t, profile.SecurityProfile.UefiSettings.VTpmEnabled)
	assert.True(t, *profile.SecurityProfile.UefiSettings.VTpmEnabled)

	require.NotNil(t, profile.StorageProfile)
	require.NotNil(t, profile.StorageProfile.OSDisk)
	require.NotNil(t, profile.StorageProfile.OSDisk.ManagedDisk)
	require.NotNil(t, profile.StorageProfile.OSDisk.ManagedDisk.SecurityProfile)
	require.NotNil(t, profile.StorageProfile.OSDisk.ManagedDisk.SecurityProfile.SecurityEncryptionType)
	assert.Equal(
		t,
		armcompute.SecurityEncryptionTypesVMGuestStateOnly,
		*profile.StorageProfile.OSDisk.ManagedDisk.SecurityProfile.SecurityEncryptionType,
	)
}

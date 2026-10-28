package scenario

import (
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCVMScenarioUsesWestUSAndDefaultValidation(t *testing.T) {
	s := newCVMScenario(
		"test",
		"description",
		config.VHDUbuntu2404CVMGen2Containerd,
		ClusterKubenet,
		cvmSNPSKU,
		false,
		armcompute.SecurityEncryptionTypesVMGuestStateOnly,
	)

	assert.Equal(t, "westus", s.Location)
	assert.True(t, s.Tags.CVM)
	assert.Equal(t, config.VHDUbuntu2404CVMGen2Containerd, s.VHD)
	assert.False(t, s.SkipDefaultValidation)
	assert.Nil(t, s.Validator)

	nbc := &datamodel.NodeBootstrappingConfiguration{
		AgentPoolProfile: &datamodel.AgentPoolProfile{},
	}
	s.BootstrapConfigMutator(nil, nbc)
	assert.Equal(t, cvmSNPSKU, nbc.AgentPoolProfile.VMSize)
}

func TestRegisteredCVMScenarios(t *testing.T) {
	expected := map[string]struct {
		vhd                    *config.Image
		useNVMe                bool
		disableLocalDNS        bool
		securityEncryptionType armcompute.SecurityEncryptionTypes
	}{
		"Ubuntu2004_CVM_SNP": {
			vhd:                    config.VHDUbuntu2004CVMGen2Containerd,
			disableLocalDNS:        true,
			securityEncryptionType: armcompute.SecurityEncryptionTypesVMGuestStateOnly,
		},
		"Ubuntu2404_CVM_SNP": {
			vhd:                    config.VHDUbuntu2404CVMGen2Containerd,
			securityEncryptionType: armcompute.SecurityEncryptionTypesVMGuestStateOnly,
		},
		"AzureLinuxV3_CVM_SNP": {
			vhd:                    config.VHDAzureLinuxV3CVMGen2,
			securityEncryptionType: armcompute.SecurityEncryptionTypesVMGuestStateOnly,
		},
		"AzureContainerLinux_CVM_SNP": {
			vhd:                    config.VHDACLCVMGen2,
			securityEncryptionType: armcompute.SecurityEncryptionTypesVMGuestStateOnly,
		},
		"Ubuntu2604Minimal_CVM_SNP": {
			vhd:                    config.VHDUbuntu2604MinimalCVMGen2Containerd,
			securityEncryptionType: armcompute.SecurityEncryptionTypesDiskWithVMGuestState,
		},
		"Ubuntu2604Minimal_CVM_TDX": {
			vhd:                    config.VHDUbuntu2604MinimalCVMGen2Containerd,
			useNVMe:                true,
			securityEncryptionType: armcompute.SecurityEncryptionTypesDiskWithVMGuestState,
		},
	}

	found := make(map[string]bool, len(expected))
	for _, s := range List() {
		want, ok := expected[s.Name]
		if !ok {
			continue
		}
		found[s.Name] = true
		assert.Equal(t, "westus", s.Location)
		assert.Equal(t, want.vhd, s.VHD)
		assert.Equal(t, want.useNVMe, s.UseNVMe)
		assert.False(t, s.SkipDefaultValidation)
		assert.Nil(t, s.Validator)

		nbc, err := baseTemplateLinux("westus", "1.36.5", "amd64")
		require.NoError(t, err)
		s.BootstrapConfigMutator(nil, nbc)
		nodeConfig, err := nbcToAKSNodeConfigV1(nbc)
		require.NoError(t, err)
		if s.AKSNodeConfigMutator != nil {
			s.AKSNodeConfigMutator(nil, nodeConfig)
		}
		if want.disableLocalDNS {
			assert.Nil(t, nbc.AgentPoolProfile.LocalDNSProfile)
			assert.Nil(t, nodeConfig.LocalDnsProfile)
			assert.True(t, s.VHD.UnsupportedLocalDns)
		} else {
			require.NotNil(t, nbc.AgentPoolProfile.LocalDNSProfile)
			assert.True(t, nbc.AgentPoolProfile.LocalDNSProfile.EnableLocalDNS)
			require.NotNil(t, nodeConfig.LocalDnsProfile)
			assert.True(t, nodeConfig.LocalDnsProfile.EnableLocalDns)
		}

		vmss := &armcompute.VirtualMachineScaleSet{}
		s.VMConfigMutator(vmss)
		assert.Equal(
			t,
			want.securityEncryptionType,
			*vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.ManagedDisk.SecurityProfile.SecurityEncryptionType,
		)
	}
	for name := range expected {
		assert.True(t, found[name], "CVM scenario %q is not registered", name)
	}
}

func TestConfigureConfidentialVM(t *testing.T) {
	osProfile := &armcompute.VirtualMachineScaleSetOSProfile{}
	vmss := &armcompute.VirtualMachineScaleSet{
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				OSProfile: osProfile,
			},
		},
	}

	configureConfidentialVM(vmss, cvmTDXSKU, armcompute.SecurityEncryptionTypesDiskWithVMGuestState)

	require.NotNil(t, vmss.SKU)
	require.NotNil(t, vmss.SKU.Name)
	assert.Equal(t, cvmTDXSKU, *vmss.SKU.Name)

	require.NotNil(t, vmss.Properties)
	require.NotNil(t, vmss.Properties.VirtualMachineProfile)
	profile := vmss.Properties.VirtualMachineProfile
	assert.Same(t, osProfile, profile.OSProfile)
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
		armcompute.SecurityEncryptionTypesDiskWithVMGuestState,
		*profile.StorageProfile.OSDisk.ManagedDisk.SecurityProfile.SecurityEncryptionType,
	)
}
